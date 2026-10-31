#include <stddef.h>
#include <stdint.h>

typedef struct vt_decoder vt_decoder;

vt_decoder *vt_new(uintptr_t handle);
int vt_configure(vt_decoder *d, int codec, const uint8_t *const *sets, const size_t *sizes, int count, int length_size);
int vt_decode(vt_decoder *d, const uint8_t *data, size_t length, int64_t pts_us);
void vt_close(vt_decoder *d);
void vt_size(void *image, int *width, int *height);
int vt_copy(void *image, uint8_t *dst, size_t dst_stride);
