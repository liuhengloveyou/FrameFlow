#!/usr/bin/env bash
# 打包飞牛 fnOS 普通应用（.fpk）
# 用法: ./scripts/pack-fnos.sh
#
# 依赖: pnpm、go、fnpack、gcc（CGO）
# 原生库: lib/ffmpeg、lib/onnxruntime → 打进 app/server/lib/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP="$ROOT/release/FrameFlow.fnos"
OUT="$ROOT/release/dist"
ARCH=amd64
FFMPEG_DIR="$ROOT/lib/ffmpeg/linux/$ARCH"
ORT_DIR="$ROOT/lib/onnxruntime/linux/$ARCH"

command -v pnpm >/dev/null || { echo "需要 pnpm"; exit 1; }
command -v go >/dev/null || { echo "需要 go"; exit 1; }
command -v fnpack >/dev/null || { echo "需要 fnpack（加入 PATH）"; exit 1; }

# macOS → linux 需要交叉 C 编译器
if [[ "$(uname -s)" == Darwin ]]; then
  if [[ -z "${CC:-}" ]]; then
    for c in x86_64-linux-musl-gcc x86_64-unknown-linux-gnu-gcc; do
      if command -v "$c" >/dev/null 2>&1; then
        export CC="$c"
        break
      fi
    done
  fi
  if [[ -z "${CC:-}" ]]; then
    echo "ERROR: macOS 交叉编译 linux 需要 C 编译器（CGO）。" >&2
    echo "  brew install filosottile/musl-cross/musl-cross" >&2
    echo "  CC=x86_64-linux-musl-gcc ./scripts/pack-fnos.sh" >&2
    exit 1
  fi
  echo "交叉编译器: $CC"
fi

echo "==> 检查原生依赖"
if [[ ! -d "$FFMPEG_DIR/include/libavcodec" ]] || ! ls "$FFMPEG_DIR/lib"/libavcodec.so* >/dev/null 2>&1; then
  echo "缺少 FFmpeg，尝试拉取…"
  "$ROOT/scripts/fetch-ffmpeg-linux.sh" "$ARCH"
fi
if ! ls "$ORT_DIR"/libonnxruntime.so* >/dev/null 2>&1; then
  echo "缺少 ONNX Runtime，尝试拉取…"
  "$ROOT/scripts/fetch-onnxruntime-linux.sh" "$ARCH"
fi
[[ -d "$FFMPEG_DIR/include/libavcodec" ]] || { echo "缺少 $FFMPEG_DIR/include"; exit 1; }
ls "$FFMPEG_DIR/lib"/libavcodec.so* >/dev/null || { echo "缺少 libavcodec.so*"; exit 1; }
ls "$ORT_DIR"/libonnxruntime.so* >/dev/null || { echo "缺少 libonnxruntime.so*"; exit 1; }

echo "==> 前端"
(cd "$ROOT/frontend" && pnpm build)

echo "==> 准备 app/server/lib 与默认配置"
mkdir -p "$APP/app/server/lib" "$OUT"
rm -f "$APP/app/server/lib"/libav*.so* \
      "$APP/app/server/lib"/libsw*.so* \
      "$APP/app/server/lib"/libonnxruntime*.so*
cp -a "$FFMPEG_DIR/lib"/. "$APP/app/server/lib/"
cp -a "$ORT_DIR"/libonnxruntime.so* "$APP/app/server/lib/"
# providers 可选但建议带上
if [[ -f "$ORT_DIR/libonnxruntime_providers_shared.so" ]]; then
  cp -a "$ORT_DIR/libonnxruntime_providers_shared.so" "$APP/app/server/lib/"
fi
# 默认配置必须打进包；安装/启动时复制到 $TRIM_PKGETC
copy_default_conf() {
  local name=$1
  local dest="$APP/app/server/$name"
  if [[ -f "$dest" ]]; then
    return 0
  fi
  local src=""
  for c in "$ROOT/configs/$name" "$ROOT/$name"; do
    if [[ -f "$c" ]]; then
      src=$c
      break
    fi
  done
  if [[ -z "$src" ]]; then
    echo "ERROR: 缺少 $name（需要 $dest 或 configs/$name）" >&2
    exit 1
  fi
  cp -a "$src" "$dest"
  echo "  copied $src -> $dest"
}
copy_default_conf frameflow.yaml
copy_default_conf passport.conf.yaml
copy_default_conf rbac_with_domains_model.conf

echo "==> Go linux/$ARCH (CGO)"
CGO_ENABLED=1 GOOS=linux GOARCH="$ARCH" \
  go build -trimpath -ldflags "-s -w" \
  -o "$APP/app/server/FrameFlow" "$ROOT"

chmod +x "$APP/app/server/FrameFlow"

echo "==> fnpack"
(cd "$APP" && fnpack build)

version=$(grep -E '^version[[:space:]]*=' "$APP/manifest" | cut -d= -f2- | xargs)
fpk="$OUT/FrameFlow_x86_v${version}.fpk"
mv -f "$APP/FrameFlow.fpk" "$fpk" 2>/dev/null \
  || mv -f "$ROOT/release/FrameFlow.fpk" "$fpk" 2>/dev/null \
  || mv -f "$ROOT/FrameFlow.fpk" "$fpk"

echo "完成: $fpk"
ls -lh "$fpk"
echo "包内原生库:"
ls "$APP/app/server/lib" | head -30
