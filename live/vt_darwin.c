
#include <stdlib.h>
#include <string.h>
#include <stdatomic.h>
#include <TargetConditionals.h>
#include <CoreFoundation/CoreFoundation.h>
#include <CoreMedia/CoreMedia.h>
#include <CoreVideo/CoreVideo.h>
#include <VideoToolbox/VideoToolbox.h>

#include "vt_darwin.h"
#include "_cgo_export.h"

// The most pictures handed to the decoder and not yet come out. A decoder that
// falls behind is told so, and the caller waits for a keyframe, rather than
// queueing seconds of video that would be shown late.
#define VT_MAX_IN_FLIGHT 8

struct vt_decoder {
	uintptr_t handle;
	VTDecompressionSessionRef session;
	CMVideoFormatDescriptionRef format;
	atomic_int in_flight;
};

static void vt_output(void *refcon, void *frame, OSStatus status, VTDecodeInfoFlags flags, CVImageBufferRef image, CMTime pts, CMTime duration) {
	vt_decoder *d = refcon;
	atomic_fetch_sub(&d->in_flight, 1);
	if (status != noErr || image == NULL) {
		return;
	}
	int64_t us = pts.timescale > 0 ? (int64_t)(((double)pts.value / pts.timescale) * 1e6 + 0.5) : 0;
	ayraVTFrame(d->handle, (void *)image, us);
}

vt_decoder *vt_new(uintptr_t handle) {
	vt_decoder *d = calloc(1, sizeof *d);
	d->handle = handle;
	return d;
}

static void vt_stop(vt_decoder *d) {
	if (d->session != NULL) {
		VTDecompressionSessionWaitForAsynchronousFrames(d->session);
		VTDecompressionSessionInvalidate(d->session);
		CFRelease(d->session);
		d->session = NULL;
	}
}

static int vt_start(vt_decoder *d) {
	vt_stop(d);
	int32_t pixel = kCVPixelFormatType_32BGRA;
	CFNumberRef number = CFNumberCreate(NULL, kCFNumberSInt32Type, &pixel);
	const void *keys[] = {kCVPixelBufferPixelFormatTypeKey};
	const void *values[] = {number};
	CFDictionaryRef attrs = CFDictionaryCreate(NULL, keys, values, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFRelease(number);

	// A Mac is asked for its hardware decoder by name. A phone decodes in
	// hardware without being asked, and the name is not available before iOS 17.
	CFDictionaryRef spec = NULL;
#if TARGET_OS_OSX
	const void *spec_keys[] = {kVTVideoDecoderSpecification_EnableHardwareAcceleratedVideoDecoder};
	const void *spec_values[] = {kCFBooleanTrue};
	spec = CFDictionaryCreate(NULL, spec_keys, spec_values, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
#endif

	VTDecompressionOutputCallbackRecord callback = {vt_output, d};
	OSStatus status = VTDecompressionSessionCreate(NULL, d->format, spec, attrs, &callback, &d->session);
	CFRelease(attrs);
	if (spec != NULL) {
		CFRelease(spec);
	}
	return (int)status;
}

int vt_configure(vt_decoder *d, int codec, const uint8_t *const *sets, const size_t *sizes, int count, int length_size) {
	vt_stop(d);
	if (d->format != NULL) {
		CFRelease(d->format);
		d->format = NULL;
	}
	OSStatus status;
	if (codec == 1) {
		status = CMVideoFormatDescriptionCreateFromH264ParameterSets(NULL, count, sets, sizes, length_size, &d->format);
	} else {
		status = CMVideoFormatDescriptionCreateFromHEVCParameterSets(NULL, count, sets, sizes, length_size, NULL, &d->format);
	}
	if (status != noErr) {
		return (int)status;
	}
	return vt_start(d);
}

int vt_decode(vt_decoder *d, const uint8_t *data, size_t length, int64_t pts_us) {
	if (d->session == NULL || d->format == NULL) {
		return -2;
	}
	if (atomic_load(&d->in_flight) >= VT_MAX_IN_FLIGHT) {
		return -1;
	}
	for (int attempt = 0; attempt < 2; attempt++) {
		CMBlockBufferRef block = NULL;
		OSStatus status = CMBlockBufferCreateWithMemoryBlock(NULL, NULL, length, NULL, NULL, 0, length, 0, &block);
		if (status != noErr) {
			return (int)status;
		}
		status = CMBlockBufferReplaceDataBytes(data, block, 0, length);
		if (status != noErr) {
			CFRelease(block);
			return (int)status;
		}
		CMSampleTimingInfo timing = {kCMTimeInvalid, CMTimeMake(pts_us, 1000000), kCMTimeInvalid};
		CMSampleBufferRef sample = NULL;
		status = CMSampleBufferCreateReady(NULL, block, d->format, 1, 1, &timing, 1, &length, &sample);
		CFRelease(block);
		if (status != noErr) {
			return (int)status;
		}
		atomic_fetch_add(&d->in_flight, 1);
		VTDecodeInfoFlags info = 0;
		status = VTDecompressionSessionDecodeFrame(d->session, sample, kVTDecodeFrame_EnableAsynchronousDecompression, NULL, &info);
		CFRelease(sample);
		if (status == noErr) {
			return 0;
		}
		atomic_fetch_sub(&d->in_flight, 1);
		// A session is invalidated when the application goes to the background;
		// it is made again from the same configuration, once.
		if (status == kVTInvalidSessionErr && attempt == 0) {
			int again = vt_start(d);
			if (again != 0) {
				return again;
			}
			continue;
		}
		return (int)status;
	}
	return -3;
}

void vt_close(vt_decoder *d) {
	vt_stop(d);
	if (d->format != NULL) {
		CFRelease(d->format);
	}
	free(d);
}

void vt_size(void *image, int *width, int *height) {
	*width = (int)CVPixelBufferGetWidth((CVPixelBufferRef)image);
	*height = (int)CVPixelBufferGetHeight((CVPixelBufferRef)image);
}

// vt_copy writes the picture as RGBA, which is the layout the engine uploads
// without converting it again. The decoder is asked for BGRA because that is the
// one packed format it produces everywhere, so the red and blue are exchanged
// here, row by row, while the buffer is locked.
int vt_copy(void *image, uint8_t *dst, size_t dst_stride) {
	CVPixelBufferRef buffer = image;
	if (CVPixelBufferLockBaseAddress(buffer, kCVPixelBufferLock_ReadOnly) != kCVReturnSuccess) {
		return -1;
	}
	size_t width = CVPixelBufferGetWidth(buffer);
	size_t height = CVPixelBufferGetHeight(buffer);
	size_t stride = CVPixelBufferGetBytesPerRow(buffer);
	const uint8_t *src = CVPixelBufferGetBaseAddress(buffer);
	for (size_t y = 0; y < height; y++) {
		const uint8_t *s = src + y * stride;
		uint8_t *o = dst + y * dst_stride;
		for (size_t x = 0; x < width; x++, s += 4, o += 4) {
			o[0] = s[2];
			o[1] = s[1];
			o[2] = s[0];
			o[3] = 255;
		}
	}
	CVPixelBufferUnlockBaseAddress(buffer, kCVPixelBufferLock_ReadOnly);
	return 0;
}
