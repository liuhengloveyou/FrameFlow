#include "ffmpeg_decode.h"

#include <libavcodec/avcodec.h>
#include <libavutil/common.h>
#include <libavutil/error.h>
#include <libavutil/imgutils.h>
#include <libavutil/mem.h>
#include <libswscale/swscale.h>
#include <string.h>

struct ffdecode_ctx {
    enum AVCodecID codec_id;
    const AVCodec *codec;
    AVCodecContext *avctx;
    AVFrame *frame;
    AVPacket *pkt;
    struct SwsContext *sws;
    int max_out_w;
    int last_src_w, last_src_h;
    int last_dst_w, last_dst_h;
    enum AVPixelFormat last_fmt;
};

static void rgb_out_dims(const ffdecode_ctx *c, int src_w, int src_h, int *dst_w, int *dst_h)
{
    if (c->max_out_w <= 0 || src_w <= 0 || src_h <= 0 || src_w <= c->max_out_w) {
        *dst_w = src_w;
        *dst_h = src_h;
        return;
    }
    *dst_w = c->max_out_w;
    *dst_h = (int)((int64_t)src_h * c->max_out_w / src_w);
    if (*dst_h < 1)
        *dst_h = 1;
}

static int ensure_sws(ffdecode_ctx *c, const AVFrame *f)
{
    int dst_w, dst_h;
    rgb_out_dims(c, f->width, f->height, &dst_w, &dst_h);

    if (c->sws && c->last_src_w == f->width && c->last_src_h == f->height &&
        c->last_fmt == (enum AVPixelFormat)f->format && c->last_dst_w == dst_w && c->last_dst_h == dst_h)
        return 0;
    sws_freeContext(c->sws);
    c->sws = sws_getContext(f->width, f->height, (enum AVPixelFormat)f->format, dst_w, dst_h,
                            AV_PIX_FMT_RGB24, SWS_BILINEAR, NULL, NULL, NULL);
    if (!c->sws)
        return AVERROR(ENOMEM);
    c->last_src_w = f->width;
    c->last_src_h = f->height;
    c->last_dst_w = dst_w;
    c->last_dst_h = dst_h;
    c->last_fmt = (enum AVPixelFormat)f->format;
    return 0;
}

static int open_codec(ffdecode_ctx *c)
{
    c->avctx = avcodec_alloc_context3(c->codec);
    if (!c->avctx)
        return AVERROR(ENOMEM);
    c->avctx->flags |= AV_CODEC_FLAG_LOW_DELAY;
    c->avctx->flags2 |= AV_CODEC_FLAG2_SHOW_ALL;
    if (avcodec_open2(c->avctx, c->codec, NULL) < 0) {
        avcodec_free_context(&c->avctx);
        return AVERROR_DECODER_NOT_FOUND;
    }
    return 0;
}

static int reset_codec(ffdecode_ctx *c)
{
    avcodec_free_context(&c->avctx);
    return open_codec(c);
}

static ffdecode_ctx *ffdecode_open2(enum AVCodecID id, int max_out_w)
{
    ffdecode_ctx *c = (ffdecode_ctx *)av_mallocz(sizeof(*c));
    if (!c)
        return NULL;

    if (max_out_w < 0)
        max_out_w = 0;
    c->max_out_w = max_out_w;
    c->codec_id = id;

    c->codec = avcodec_find_decoder(id);
    if (!c->codec) {
        av_free(c);
        return NULL;
    }
    if (open_codec(c) < 0) {
        av_free(c);
        return NULL;
    }

    c->frame = av_frame_alloc();
    c->pkt = av_packet_alloc();
    if (!c->frame || !c->pkt) {
        av_frame_free(&c->frame);
        av_packet_free(&c->pkt);
        avcodec_free_context(&c->avctx);
        av_free(c);
        return NULL;
    }

    c->last_fmt = AV_PIX_FMT_NONE;
    return c;
}

ffdecode_ctx *ffdecode_open_h264_maxw(int max_out_w) { return ffdecode_open2(AV_CODEC_ID_H264, max_out_w); }

ffdecode_ctx *ffdecode_open_hevc_maxw(int max_out_w) { return ffdecode_open2(AV_CODEC_ID_HEVC, max_out_w); }

ffdecode_ctx *ffdecode_open_h264(void) { return ffdecode_open_h264_maxw(0); }

ffdecode_ctx *ffdecode_open_hevc(void) { return ffdecode_open_hevc_maxw(0); }

void ffdecode_close(ffdecode_ctx *c)
{
    if (!c)
        return;
    sws_freeContext(c->sws);
    av_frame_free(&c->frame);
    av_packet_free(&c->pkt);
    avcodec_free_context(&c->avctx);
    av_free(c);
}

static void free_rgb_out(uint8_t **rgb_out)
{
    if (rgb_out && *rgb_out) {
        av_free(*rgb_out);
        *rgb_out = NULL;
    }
}

static int frame_to_rgb(ffdecode_ctx *c, uint8_t **rgb_out, int *w_out, int *h_out)
{
    int dst_w, dst_h;
    int ret = ensure_sws(c, c->frame);
    if (ret < 0)
        return ret;

    rgb_out_dims(c, c->frame->width, c->frame->height, &dst_w, &dst_h);

    {
        int rgb_sz = av_image_get_buffer_size(AV_PIX_FMT_RGB24, dst_w, dst_h, 1);
        uint8_t *rgb = (uint8_t *)av_malloc((size_t)rgb_sz + AV_INPUT_BUFFER_PADDING_SIZE);
        if (!rgb)
            return AVERROR(ENOMEM);
        memset(rgb + rgb_sz, 0, AV_INPUT_BUFFER_PADDING_SIZE);

        {
            uint8_t *pdst[4] = { rgb };
            int ls[4] = { dst_w * 3, 0, 0, 0 };
            sws_scale(c->sws, (const uint8_t *const *)c->frame->data, c->frame->linesize, 0, c->frame->height,
                      pdst, ls);
        }

        if (*rgb_out)
            av_free(*rgb_out);
        *rgb_out = rgb;
        *w_out = dst_w;
        *h_out = dst_h;
    }
    return 0;
}

static int receive_frames(ffdecode_ctx *c, uint8_t **rgb_out, int *w_out, int *h_out, int *got_frame)
{
    for (;;) {
        int ret;
        av_frame_unref(c->frame);
        ret = avcodec_receive_frame(c->avctx, c->frame);
        if (ret == AVERROR(EAGAIN) || ret == AVERROR_EOF)
            return 0;
        if (ret < 0)
            return ret;
        ret = frame_to_rgb(c, rgb_out, w_out, h_out);
        if (ret < 0)
            return ret;
        *got_frame = 1;
    }
}

static int decode_au(ffdecode_ctx *c, const uint8_t *au, int au_size, uint8_t **rgb_out, int *w_out,
                     int *h_out, int *got_frame)
{
    int ret;

    if (au_size <= 0)
        return 0;

    av_packet_unref(c->pkt);
    if (av_new_packet(c->pkt, au_size) < 0)
        return AVERROR(ENOMEM);
    memcpy(c->pkt->data, au, (size_t)au_size);

    ret = avcodec_send_packet(c->avctx, c->pkt);
    av_packet_unref(c->pkt);
    if (ret < 0 && ret != AVERROR(EAGAIN))
        return ret;

    return receive_frames(c, rgb_out, w_out, h_out, got_frame);
}

static int drain_and_reset(ffdecode_ctx *c, uint8_t **rgb_out, int *w_out, int *h_out, int *got_frame)
{
    int ret = avcodec_send_packet(c->avctx, NULL);
    if (ret < 0 && ret != AVERROR_EOF)
        return ret;
    ret = receive_frames(c, rgb_out, w_out, h_out, got_frame);
    if (ret < 0)
        return ret;
    return reset_codec(c);
}

static int find_start_code(const uint8_t *b, int n, int from, int *sc_len)
{
    int i;
    for (i = from; i + 3 <= n; i++) {
        if (b[i] != 0 || b[i + 1] != 0)
            continue;
        if (b[i + 2] == 1) {
            *sc_len = 3;
            return i;
        }
        if (i + 4 <= n && b[i + 2] == 0 && b[i + 3] == 1) {
            *sc_len = 4;
            return i;
        }
    }
    return -1;
}

static int nal_hdr(const uint8_t *b, int n, int sc_off, int sc_len, uint8_t *hdr)
{
    int p = sc_off + sc_len;
    if (p >= n)
        return -1;
    *hdr = b[p];
    return 0;
}

static int h264_is_vcl(uint8_t hdr) { return (hdr & 0x1F) >= 1 && (hdr & 0x1F) <= 5; }

static int h264_is_aud(uint8_t hdr) { return (hdr & 0x1F) == 9; }

static int h265_nal_type(uint8_t hdr) { return (hdr >> 1) & 0x3F; }

static int h265_is_vcl(uint8_t hdr) { return h265_nal_type(hdr) <= 31; }

static int h265_is_aud(uint8_t hdr) { return h265_nal_type(hdr) == 35; }

int ffdecode_feed(ffdecode_ctx *c, const uint8_t *data, size_t len, uint8_t **rgb_out, int *w_out,
                  int *h_out)
{
    int got_frame = 0;
    int n, pos, au_start, has_vcl;
    int sc_len;
    int r;

    *rgb_out = NULL;

    if (!data || len == 0)
        return 1;

    n = (int)len;
    pos = find_start_code(data, n, 0, &sc_len);
    if (pos < 0)
        return 1;

    au_start = pos;
    has_vcl = 0;

    while (pos < n) {
        int sc0, sc1, sc_len0, sc_len1;
        uint8_t hdr;
        int is_vcl, is_aud;

        sc0 = find_start_code(data, n, pos, &sc_len0);
        if (sc0 < 0)
            break;
        if (nal_hdr(data, n, sc0, sc_len0, &hdr) < 0)
            break;

        if (c->codec_id == AV_CODEC_ID_HEVC) {
            is_vcl = h265_is_vcl(hdr);
            is_aud = h265_is_aud(hdr);
        } else {
            is_vcl = h264_is_vcl(hdr);
            is_aud = h264_is_aud(hdr);
        }

        if ((is_aud || (is_vcl && has_vcl)) && sc0 > au_start) {
            r = decode_au(c, data + au_start, sc0 - au_start, rgb_out, w_out, h_out, &got_frame);
            if (r < 0) {
                free_rgb_out(rgb_out);
                reset_codec(c);
                return r;
            }
            au_start = sc0;
            has_vcl = 0;
        }
        if (is_vcl)
            has_vcl = 1;

        sc1 = find_start_code(data, n, sc0 + sc_len0, &sc_len1);
        if (sc1 < 0) {
            r = decode_au(c, data + au_start, n - au_start, rgb_out, w_out, h_out, &got_frame);
            if (r < 0) {
                free_rgb_out(rgb_out);
                reset_codec(c);
                return r;
            }
            break;
        }
        pos = sc1;
    }

    if (!got_frame) {
        r = drain_and_reset(c, rgb_out, w_out, h_out, &got_frame);
        if (r < 0) {
            free_rgb_out(rgb_out);
            reset_codec(c);
            return r;
        }
    } else {
        r = reset_codec(c);
        if (r < 0) {
            free_rgb_out(rgb_out);
            return r;
        }
    }

    return got_frame ? 0 : 1;
}
