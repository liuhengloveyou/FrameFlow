#!/usr/bin/env bash
# 把当前 OS/架构的 FFmpeg / ONNX 共享库链到仓库根 lib/，便于：
#   CGO_ENABLED=1 go build -o FrameFlow .
#   ./FrameFlow
# （Linux 二进制 rpath 为 $ORIGIN/lib；macOS 由 vision.ort_lib / 环境变量加载 ORT）
#
# 布局（与平台无关）:
#   lib/ffmpeg/{linux|darwin}/{amd64|arm64}/...
#   lib/onnxruntime/{linux|darwin}/{amd64|arm64}/libonnxruntime.*
#   lib/libonnxruntime.{so|dylib}  ← 本脚本生成的运行时入口
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux) OS=linux ;;
  darwin) OS=darwin ;;
  *) echo "不支持的系统: $OS" >&2; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "不支持的架构: $ARCH" >&2; exit 1 ;;
esac

FF="$ROOT/lib/ffmpeg/$OS/$ARCH/lib"
ORT="$ROOT/lib/onnxruntime/$OS/$ARCH"
DEST="$ROOT/lib"

link_ffmpeg() {
  if [[ ! -d "$FF" ]]; then
    if [[ "$OS" == darwin ]]; then
      echo "INFO: 无 ${FF}（macOS 本地可用 Homebrew FFmpeg / pkg-config）"
    else
      echo "缺少 ${FF}，请先: ./scripts/fetch-ffmpeg-linux.sh ${ARCH}" >&2
      exit 1
    fi
    return
  fi
  echo "==> 链接 FFmpeg (${OS}/${ARCH}) -> lib/"
  shopt -s nullglob
  local f base
  for f in "$FF"/*.so* "$FF"/*.dylib*; do
    [[ -e "$f" ]] || continue
    base="$(basename "$f")"
    ln -sfn "ffmpeg/$OS/$ARCH/lib/$base" "$DEST/$base"
  done
}

link_ort() {
  if [[ ! -d "$ORT" ]]; then
    echo "WARN: 无 ${ORT}（vision 推理需 ONNX Runtime）"
    if [[ "$OS" == darwin ]]; then
      echo "      请先: ./scripts/fetch-onnxruntime-osx.sh ${ARCH}"
    else
      echo "      请先: ./scripts/fetch-onnxruntime-linux.sh ${ARCH}"
    fi
    return
  fi
  echo "==> 链接 ONNX Runtime (${OS}/${ARCH}) -> lib/"
  shopt -s nullglob
  local f base
  for f in "$ORT"/libonnxruntime.so* "$ORT"/libonnxruntime*.dylib "$ORT"/libonnxruntime_providers_shared.so; do
    [[ -e "$f" ]] || continue
    base="$(basename "$f")"
    ln -sfn "onnxruntime/$OS/$ARCH/$base" "$DEST/$base"
  done
}

link_ffmpeg
link_ort

echo "完成 (${OS}/${ARCH})。"
if [[ "$OS" == darwin ]]; then
  echo "  export ONNXRUNTIME_SHARED_LIBRARY_PATH=\"${DEST}/libonnxruntime.dylib\""
else
  echo "  export ONNXRUNTIME_SHARED_LIBRARY_PATH=\"${DEST}/libonnxruntime.so\""
fi
echo "  CGO_ENABLED=1 go build -o FrameFlow . && ./FrameFlow"
