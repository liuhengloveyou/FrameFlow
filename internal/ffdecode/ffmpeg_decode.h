#ifndef FRAMEFLOW_FFDECODE_FFMPEG_DECODE_H
#define FRAMEFLOW_FFDECODE_FFMPEG_DECODE_H

#include <stddef.h>
#include <stdint.h>

typedef struct ffdecode_ctx ffdecode_ctx;

/* max_out_w <= 0: RGB 与源同尺寸；>0 且源更宽时缩放到宽 max_out_w、高按比例。 */
ffdecode_ctx *ffdecode_open_h264_maxw(int max_out_w);
ffdecode_ctx *ffdecode_open_hevc_maxw(int max_out_w);
ffdecode_ctx *ffdecode_open_h264(void);
ffdecode_ctx *ffdecode_open_hevc(void);
void ffdecode_close(ffdecode_ctx *ctx);

/*
 * 一次性解码 Annex-B 样本（建议含 SPS/PPS + 自 IDR 起的完整可解码片段）。
 * 按访问单元送入 libavcodec，必要时 drain 重排延迟，然后重置解码器。
 *
 * Returns:
 *   0  — 至少解出一帧；*rgb_out 为最后一帧（av_malloc，调用方 av_free）
 *   1  — 无帧
 *   <0 — AVERROR
 */
int ffdecode_feed(ffdecode_ctx *ctx, const uint8_t *data, size_t len, uint8_t **rgb_out, int *w_out,
                  int *h_out);

#endif
