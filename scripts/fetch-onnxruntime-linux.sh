#!/usr/bin/env bash
# 下载 Linux 预编译 ONNX Runtime 到 lib/onnxruntime/linux/{amd64|arm64}/
# 用法:
#   ./scripts/fetch-onnxruntime-linux.sh           # 默认 amd64
#   ./scripts/fetch-onnxruntime-linux.sh arm64
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARCH="${1:-amd64}"
VER="1.26.0"

case "$ARCH" in
  amd64|x86_64)
    ARCH=amd64
    ASSET="onnxruntime-linux-x64-${VER}.tgz"
    ;;
  arm64|aarch64)
    ARCH=arm64
    ASSET="onnxruntime-linux-aarch64-${VER}.tgz"
    ;;
  *)
    echo "用法: $0 [amd64|arm64]" >&2
    exit 1
    ;;
esac

DEST="$ROOT/lib/onnxruntime/linux/$ARCH"
BASE_URL="https://github.com/microsoft/onnxruntime/releases/download/v${VER}/${ASSET}"
# 国内可设: export GITHUB_PROXY=https://ghfast.top/
URL="${GITHUB_PROXY:-}${BASE_URL}"

if ls "$DEST"/libonnxruntime.so* >/dev/null 2>&1; then
  echo "已存在: $DEST （跳过下载）"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> 下载 $ASSET"
curl -fL --retry 3 -o "$TMP/$ASSET" "$URL"

echo "==> 解压并安装到 $DEST"
tar -xzf "$TMP/$ASSET" -C "$TMP"
SRC="$(find "$TMP" -maxdepth 1 -type d -name 'onnxruntime-*' | head -1)"
[[ -n "$SRC" ]] || { echo "解压后未找到 onnxruntime-* 目录" >&2; exit 1; }

rm -rf "$DEST"
mkdir -p "$DEST"
cp -a "$SRC/lib"/. "$DEST/"
# 保留一份头文件便于排查（非编译必需）
if [[ -d "$SRC/include" ]]; then
  cp -a "$SRC/include" "$DEST/"
fi

echo "v${VER} ${ASSET}" > "$DEST/VERSION.txt"
echo "完成: $DEST"
ls -lh "$DEST" | head -20
echo "下一步: ./scripts/prepare-dev-libs.sh"