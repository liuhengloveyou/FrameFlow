#!/usr/bin/env bash
# 将本地 MP4 经 RTSP ANNOUNCE 推到 FrameFlow（默认 data/datasets/input-1.mp4 → live）。
#
# 默认：仅推 H264 constrained baseline（无 B 帧、无音频），保证浏览器 MSE/WebRTC 可预览。
# input-1.mp4 原片为 High+B 帧 + AAC 5.1；-c copy 或带 PCM/FLAC 音轨时 MSE 易失败。
#
# 用法:
#   ./scripts/push-test-rtsp.sh -k ingest_xxx
#   ./scripts/push-test-rtsp.sh -k cam01 -a   # 额外推 AAC 立体声
#   ./scripts/push-test-rtsp.sh -k cam01 -c   # 原样 copy（可能无法预览）
#   ./scripts/push-test-rtsp.sh -1            # 只推一遍
#   HOST=192.168.1.10 PORT=8554 ./scripts/push-test-rtsp.sh
#
# 依赖: ffmpeg。推流前请启用 RTSP 监听，且目标 stream_key 已存在。

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VIDEO="${VIDEO:-$ROOT/data/datasets/input-1.mp4}"
HOST="${HOST:-127.0.0.1}"
PORT="${PORT:-8554}"
STREAM_KEY="${STREAM_KEY:-live}"
PUSH_KEY="${PUSH_KEY:-}"
LOOP="${LOOP:-1}"          # 1=无限循环；0=播完退出
REENCODE="${REENCODE:-1}"  # 1=转码（默认，可预览）；0=copy
WITH_AUDIO="${WITH_AUDIO:-0}" # 1=转码时带 AAC 立体声；0=仅视频（默认）
TRANSPORT="${TRANSPORT:-tcp}"
RETRY_SEC="${RETRY_SEC:-1}"

usage() {
  cat <<'EOF'
用法: push-test-rtsp.sh [选项]

  -i FILE     输入视频（默认 data/datasets/input-1.mp4）
  -H HOST     RTSP 主机（默认 127.0.0.1）
  -p PORT     RTSP 端口（默认 8554）
  -k KEY      stream_key（默认 live）
  -s SECRET   推流密钥 push_key（可选；URL 查询参数）
  -t PROTO    rtsp_transport：tcp（默认）| udp
  -1          只推一遍，不循环
  -e          转码 constrained baseline H264（默认，保证预览可播）
  -a          转码时附带 AAC 立体声（默认无音频，MSE 更稳）
  -c          封装拷贝（原片 5.1/B 帧时浏览器可能播不了）
  -h          帮助

环境变量: VIDEO HOST PORT STREAM_KEY PUSH_KEY TRANSPORT LOOP=0|1 REENCODE=0|1 WITH_AUDIO=0|1 RETRY_SEC
EOF
}

while getopts "i:H:p:k:s:t:1each" opt; do
  case "$opt" in
    i) VIDEO="$OPTARG" ;;
    H) HOST="$OPTARG" ;;
    p) PORT="$OPTARG" ;;
    k) STREAM_KEY="$OPTARG" ;;
    s) PUSH_KEY="$OPTARG" ;;
    t) TRANSPORT="$OPTARG" ;;
    1) LOOP=0 ;;
    e) REENCODE=1 ;;
    a) WITH_AUDIO=1 ;;
    c) REENCODE=0 ;;
    h) usage; exit 0 ;;
    *) usage; exit 1 ;;
  esac
done

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "error: 未找到 ffmpeg" >&2
  exit 1
fi
if [[ ! -f "$VIDEO" ]]; then
  echo "error: 输入文件不存在: $VIDEO" >&2
  exit 1
fi

URL="rtsp://${HOST}:${PORT}/${STREAM_KEY}"
if [[ -n "$PUSH_KEY" ]]; then
  URL="${URL}?push_key=${PUSH_KEY}"
fi

enc_desc='copy'
if [[ "$REENCODE" == "1" ]]; then
  if [[ "$WITH_AUDIO" == "1" ]]; then
    enc_desc='libx264 constrained baseline + aac stereo'
  else
    enc_desc='libx264 constrained baseline（无音频）'
  fi
fi

echo "==> 推流: $VIDEO"
echo "    目标: $URL"
echo "    传输: $TRANSPORT"
echo "    循环: $([[ "$LOOP" == "1" ]] && echo "无限（片源循环 + 断线重推）" || echo 否)  编码: $enc_desc"
echo "    预览: 浏览器监控页 src=${STREAM_KEY}"
echo

build_args() {
  args=(-hide_banner -loglevel info)
  if [[ "$LOOP" == "1" ]]; then
    args+=(-stream_loop -1)
  fi
  args+=(-re -i "$VIDEO")

  if [[ "$REENCODE" == "1" ]]; then
    # constrained baseline + 强制无 B 帧；避免个别 ffmpeg 落到 Main 导致 MSE 解码失败
    args+=(
      -vf "fps=25,scale=1280:-2"
      -c:v libx264 -preset veryfast -tune zerolatency
      -pix_fmt yuv420p
      -profile:v baseline -level 3.1
      -x264-params "profile=baseline:level=3.1:bframes=0:cabac=0:ref=1:aud=0:scenecut=0:keyint=50:min-keyint=50"
      -bf 0 -g 50 -keyint_min 50 -sc_threshold 0
      -b:v 2000k -maxrate 2000k -bufsize 4000k
    )
    if [[ "$WITH_AUDIO" == "1" ]]; then
      # 显式 AAC-LC 立体声，避免 5.1/PCE 进 MSE
      args+=(-c:a aac -profile:a aac_low -ar 44100 -ac 2 -b:a 128k)
    else
      args+=(-an)
    fi
  else
    args+=(-c copy)
  fi

  args+=(-rtsp_transport "$TRANSPORT" -f rtsp "$URL")
}

build_args

if [[ "$LOOP" != "1" ]]; then
  exec ffmpeg "${args[@]}"
fi

# 无限循环：片源内 -stream_loop -1；ffmpeg 退出（断线/错误）后自动重推
trap 'echo; echo "==> 已停止推流"; exit 0' INT TERM
n=0
while true; do
  n=$((n + 1))
  [[ "$n" -gt 1 ]] && echo "==> 第 ${n} 次重推（${RETRY_SEC}s 后）…" && sleep "$RETRY_SEC"
  ffmpeg "${args[@]}" || true
done
