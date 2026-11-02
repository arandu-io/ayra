package io.arandu.ayra.live;

import android.media.Image;
import android.media.MediaCodec;
import android.media.MediaFormat;
import android.os.Build;

import java.nio.ByteBuffer;

/**
 * AyraDecoder decodes the live video with the phone's hardware decoder.
 *
 * Pictures come in as the Go half writes them: it asks for an input buffer,
 * copies the picture straight into it and queues it, so the picture is copied
 * once between the socket and the codec. Frames come out on a thread of their
 * own and are handed to Go while the codec's image is still open, so the
 * conversion to the picture the window draws reads the codec's memory in place.
 */
public final class AyraDecoder {
	private static final long INPUT_TIMEOUT_US = 0;
	private static final long OUTPUT_TIMEOUT_US = 20000;
	private static final int FLEXIBLE_YUV = 0x7F420888;

	private final long handle;
	private MediaCodec codec;
	private Thread drain;
	private volatile boolean running;
	private volatile boolean failed;
	private int pendingInput = -1;
	private String reason = "";

	private AyraDecoder(long handle) {
		this.handle = handle;
	}

	public static AyraDecoder create(long handle) {
		return new AyraDecoder(handle);
	}

	/**
	 * configure starts the codec for a stream. csd holds the parameter sets with
	 * their start codes, in the order the codec wants them: one array for H.264
	 * sequence and picture sets each, one for H.265 with all three. It answers
	 * an empty string on success and the reason otherwise.
	 */
	public synchronized String configure(String mime, int width, int height, byte[][] csd) {
		stop();
		try {
			MediaFormat format = MediaFormat.createVideoFormat(mime, width, height);
			for (int i = 0; i < csd.length; i++) {
				format.setByteBuffer("csd-" + i, ByteBuffer.wrap(csd[i]));
			}
			format.setInteger(MediaFormat.KEY_COLOR_FORMAT, FLEXIBLE_YUV);
			if (Build.VERSION.SDK_INT >= 23) {
				format.setInteger(MediaFormat.KEY_PRIORITY, 0);
			}
			if (Build.VERSION.SDK_INT >= 30) {
				format.setInteger(MediaFormat.KEY_LOW_LATENCY, 1);
			}
			codec = MediaCodec.createDecoderByType(mime);
			codec.configure(format, null, null, 0);
			codec.start();
		} catch (Exception e) {
			release();
			return String.valueOf(e.getMessage() == null ? e.toString() : e.getMessage());
		}
		failed = false;
		reason = "";
		running = true;
		final MediaCodec current = codec;
		drain = new Thread(new Runnable() {
			@Override public void run() {
				drain(current);
			}
		}, "ayra-decoder");
		drain.start();
		return "";
	}

	/**
	 * acquire answers a buffer to write the next picture into, or null when the
	 * codec has no room for one or has failed; failed() tells which.
	 */
	public synchronized ByteBuffer acquire() {
		if (codec == null || failed) {
			return null;
		}
		try {
			int index = codec.dequeueInputBuffer(INPUT_TIMEOUT_US);
			if (index < 0) {
				return null;
			}
			pendingInput = index;
			ByteBuffer buffer = codec.getInputBuffer(index);
			buffer.clear();
			return buffer;
		} catch (Exception e) {
			fail(e);
			return null;
		}
	}

	/** queue hands the codec the picture written into the buffer acquire gave. */
	public synchronized boolean queue(int length, long ptsUs, boolean key) {
		if (codec == null || pendingInput < 0 || failed) {
			return false;
		}
		try {
			codec.queueInputBuffer(pendingInput, 0, length, ptsUs, key ? MediaCodec.BUFFER_FLAG_KEY_FRAME : 0);
			pendingInput = -1;
			return true;
		} catch (Exception e) {
			fail(e);
			return false;
		}
	}

	public boolean failed() {
		return failed;
	}

	public String reason() {
		return reason;
	}

	private void fail(Exception e) {
		reason = e.toString();
		failed = true;
	}

	private void drain(MediaCodec c) {
		MediaCodec.BufferInfo info = new MediaCodec.BufferInfo();
		while (running) {
			int index;
			try {
				index = c.dequeueOutputBuffer(info, OUTPUT_TIMEOUT_US);
			} catch (Exception e) {
				if (running) {
					fail(e);
				}
				return;
			}
			if (index < 0) {
				continue;
			}
			try {
				if (info.size > 0 && (info.flags & MediaCodec.BUFFER_FLAG_CODEC_CONFIG) == 0) {
					deliver(c, index, info.presentationTimeUs);
				}
			} catch (Exception e) {
				fail(e);
			} finally {
				try {
					c.releaseOutputBuffer(index, false);
				} catch (Exception e) {
					// The codec was stopped under the thread.
				}
			}
		}
	}

	private void deliver(MediaCodec c, int index, long ptsUs) {
		Image image = c.getOutputImage(index);
		if (image == null) {
			return;
		}
		try {
			Image.Plane[] planes = image.getPlanes();
			android.graphics.Rect crop = image.getCropRect();
			onFrame(handle,
				planes[0].getBuffer(), planes[1].getBuffer(), planes[2].getBuffer(),
				planes[0].getRowStride(), planes[1].getRowStride(), planes[1].getPixelStride(),
				crop.left, crop.top, crop.width(), crop.height(), ptsUs);
		} finally {
			image.close();
		}
	}

	/** close stops the codec and releases it. */
	public synchronized void close() {
		stop();
	}

	private void stop() {
		running = false;
		if (drain != null) {
			try {
				drain.join(500);
			} catch (InterruptedException e) {
				Thread.currentThread().interrupt();
			}
			drain = null;
		}
		release();
	}

	private void release() {
		pendingInput = -1;
		if (codec != null) {
			try {
				codec.stop();
			} catch (Exception e) {
				// Already stopped, or never started.
			}
			codec.release();
			codec = null;
		}
	}

	/**
	 * onFrame is Go reading one decoded picture: the three planes of the image
	 * with their strides, the crop that holds the picture, and its presentation
	 * time. The buffers are valid until it returns.
	 */
	private static native void onFrame(long handle, ByteBuffer y, ByteBuffer u, ByteBuffer v,
		int yStride, int uvStride, int uvPixelStride, int cropX, int cropY, int width, int height, long ptsUs);
}
