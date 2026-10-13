#!/usr/bin/env bash
# 下载 macOS 预编译 ONNX Runtime 到 lib/onnxruntime/darwin/{arm64|amd64}/
# 用法:
#   ./scripts/fetch-onnxruntime-osx.sh           # 默认 arm64（Apple Silicon）
#   ./scripts/fetch-onnxruntime-osx.sh amd64     # Intel
# 安装后执行 ./scripts/prepare-dev-libs.sh 链到 lib/libonnxruntime.dylib
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ARCH="${1:-arm64}"
VER="1.30.0"

case "$ARCH" in
  arm64|aarch64)
    ARCH=arm64
    ASSET="onnxruntime-osx-arm64-${VER}.tgz"
    ;;
  amd64|x64|x86_64)
    ARCH=amd64
    ASSET="onnxruntime-osx-x86_64-${VER}.tgz"
    ;;
  *)
    echo "用法: $0 [arm64|amd64]" >&2
    exit 1
    ;;
esac

DEST="$ROOT/lib/onnxruntime/darwin/$ARCH"
BASE_URL="https://github.com/microsoft/onnxruntime/releases/download/v${VER}/${ASSET}"
URL="${GITHUB_PROXY:-}${BASE_URL}"

if ls "$DEST"/libonnxruntime*.dylib >/dev/null 2>&1; then
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
# 与 linux 脚本一致：把包内 lib/ 内容摊平到 DEST（不拷 dSYM）
shopt -s nullglob
for f in "$SRC/lib"/libonnxruntime*.dylib; do
  cp -a "$f" "$DEST/"
done
# 版本化 dylib 的同名软链若在包内是相对链，cp -a 已保留；无 libonnxruntime.dylib 时补一条
if [[ ! -e "$DEST/libonnxruntime.dylib" ]]; then
  ver="$(ls "$DEST"/libonnxruntime.*.dylib 2>/dev/null | head -1 || true)"
  [[ -n "$ver" ]] && ln -sfn "$(basename "$ver")" "$DEST/libonnxruntime.dylib"
fi
if [[ -d "$SRC/include" ]]; then
  cp -a "$SRC/include" "$DEST/"
fi
if [[ -d "$SRC/lib/cmake" ]]; then
  cp -a "$SRC/lib/cmake" "$DEST/"
fi
if [[ -d "$SRC/lib/pkgconfig" ]]; then
  cp -a "$SRC/lib/pkgconfig" "$DEST/"
fi

echo "v${VER} ${ASSET}" > "$DEST/VERSION.txt"
echo "完成: $DEST"
ls -lh "$DEST" | head -20
echo "下一步: ./scripts/prepare-dev-libs.sh"
