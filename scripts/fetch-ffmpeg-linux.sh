#!/usr/bin/env bash
# 下载 Linux 预编译 FFmpeg 共享库到 lib/ffmpeg/linux/{amd64|arm64}/
# 用法:
#   ./scripts/fetch-ffmpeg-linux.sh           # 默认 amd64
#   ./scripts/fetch-ffmpeg-linux.sh arm64
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARCH="${1:-amd64}"

case "$ARCH" in
  amd64|x86_64)
    ARCH=amd64
    ASSET="ffmpeg-n8.1-latest-linux64-gpl-shared-8.1.tar.xz"
    ;;
  arm64|aarch64)
    ARCH=arm64
    ASSET="ffmpeg-n8.1-latest-linuxarm64-gpl-shared-8.1.tar.xz"
    ;;
  *)
    echo "用法: $0 [amd64|arm64]" >&2
    exit 1
    ;;
esac

DEST="$ROOT/lib/ffmpeg/linux/$ARCH"
BASE_URL="https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/${ASSET}"
# 国内可设: export GITHUB_PROXY=https://ghfast.top/
URL="${GITHUB_PROXY:-}${BASE_URL}"

if [[ -d "$DEST/include/libavcodec" && -d "$DEST/lib" ]] && ls "$DEST/lib"/libavcodec.so* >/dev/null 2>&1; then
  echo "已存在: $DEST （跳过下载）"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> 下载 $ASSET"
curl -fL --retry 3 -o "$TMP/$ASSET" "$URL"

echo "==> 解压并安装到 $DEST"
tar -xJf "$TMP/$ASSET" -C "$TMP"
SRC="$(find "$TMP" -maxdepth 1 -type d -name 'ffmpeg-*' | head -1)"
[[ -n "$SRC" ]] || { echo "解压后未找到 ffmpeg-* 目录" >&2; exit 1; }

rm -rf "$DEST"
mkdir -p "$DEST"
cp -a "$SRC/include" "$DEST/"
mkdir -p "$DEST/lib"
for base in libavcodec libavutil libswscale libavformat libswresample; do
  for f in "$SRC/lib/${base}"*; do
    [[ -e "$f" ]] || continue
    cp -a "$f" "$DEST/lib/"
  done
done

echo "$ASSET" > "$DEST/VERSION.txt"
echo "完成: $DEST"
ls -lh "$DEST/lib" | head -20
