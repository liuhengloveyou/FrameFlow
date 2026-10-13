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

/*
 * 持续（流式）解码：不在每次 feed 后重置解码器，保持参考帧状态。
 * 调用方须持续送入后续 AU（增量样本），成本与本次送入的帧数成正比，
 * 而非整个 GOP；适合「周期性取最新一帧」的实时链路。
 */
ffdecode_ctx *ffdecode_open_h264_stream(int max_out_w);
ffdecode_ctx *ffdecode_open_hevc_stream(int max_out_w);

/*
 * 增量送入：把 data 中所有访问单元按序送进持久解码器。
 *   want_rgb = 0：只推进解码状态，跳过 swscale/RGB 分配与拷贝（省 CPU 与内存带宽）。
 * Returns:
 *   0  — 至少解出一帧（want_rgb=1 时 *rgb_out 为最后一帧，av_malloc，调用方 av_free）
 *   1  — 需更多输入 / 无帧
 *   <0 — AVERROR（内部已重置解码器，调用方须从下一个 IDR 重新对齐）
 */
int ffdecode_feed_stream(ffdecode_ctx *ctx, const uint8_t *data, size_t len, int want_rgb,
                         uint8_t **rgb_out, int *w_out, int *h_out);


#endif
