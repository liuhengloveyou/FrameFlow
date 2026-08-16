// Package ffdecode decodes H.264 / H.265 Annex-B video to RGB24 using FFmpeg's libavcodec
// and libswscale, via CGO and a small C wrapper (ffmpeg_decode.c).
//
// Linux：链接仓库内预编译库 lib/ffmpeg/linux/{amd64|arm64}/（见 scripts/fetch-ffmpeg-linux.sh），
// 运行时从二进制旁 lib/（$ORIGIN/lib）加载，用户机不必安装系统 FFmpeg 开发包。
//
//	./scripts/fetch-ffmpeg-linux.sh
//	CGO_ENABLED=1 go build .
//
// 非 Linux（如 macOS 本地开发）走 pkg-config（Homebrew ffmpeg）。
//
// With CGO_ENABLED=0, [New] returns [ErrCGODisabled].
//
// This package is independent of internal/ffmpeg (subprocess CLI); that code is unchanged.
package ffdecode
