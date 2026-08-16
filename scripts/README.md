# Scripts

FrameFlow 仓库脚本用法。国内下载 GitHub 资源慢时可设：

```bash
export GITHUB_PROXY=https://ghfast.top/
```

## 本地开发：原生依赖

预编译库目录：

```
lib/ffmpeg/{linux|darwin}/{amd64|arm64}/
lib/onnxruntime/{linux|darwin}/{amd64|arm64}/
lib/libonnxruntime.{so|dylib}   # prepare-dev-libs.sh 生成的入口
```

### 拉取

```bash
# Linux FFmpeg（默认 amd64）
./scripts/fetch-ffmpeg-linux.sh
./scripts/fetch-ffmpeg-linux.sh arm64

# Linux ONNX Runtime（默认 amd64，钉 1.26.0）
./scripts/fetch-onnxruntime-linux.sh
./scripts/fetch-onnxruntime-linux.sh arm64

# macOS ONNX Runtime（默认 arm64）
./scripts/fetch-onnxruntime-osx.sh
./scripts/fetch-onnxruntime-osx.sh amd64
```

macOS 本地解码可用 Homebrew FFmpeg（`pkg-config`），不必拉 `lib/ffmpeg/darwin/`。

### 链接到 `lib/`

```bash
./scripts/prepare-dev-libs.sh
```

按当前 OS/架构把 FFmpeg / ORT 软链到仓库根 `lib/`。之后：

```bash
export ONNXRUNTIME_SHARED_LIBRARY_PATH="$PWD/lib/libonnxruntime.dylib"  # Linux 用 .so
CGO_ENABLED=1 go build -o FrameFlow .
./FrameFlow
```

配置里 `vision.ort_lib: lib/libonnxruntime.dylib`（Linux 为 `.so`）指向同一入口。

## 测试推流

将本地视频经 RTSP ANNOUNCE **无限循环**推到本机 FrameFlow（默认 `data/datasets/input.mp4` → `live`）。默认转码为 **H264 constrained baseline（无 B 帧、无音频）**，保证浏览器 MSE/WebRTC 可预览；片源 `-stream_loop -1`，进程退出后自动重推。

```bash
./scripts/push-test-rtsp.sh -k ingest_xxx   # 默认仅视频 baseline，可预览
./scripts/push-test-rtsp.sh -k live -a     # 额外带 AAC 立体声
./scripts/push-test-rtsp.sh -k live -1     # 只推一遍
./scripts/push-test-rtsp.sh -k cam01 -c    # 原样 copy（原片 5.1/B 帧可能播不了）
./scripts/push-test-rtsp.sh -h
```

常用环境变量：`VIDEO` `HOST` `PORT`（默认 8554）`STREAM_KEY` `PUSH_KEY` `TRANSPORT` `REENCODE=0|1` `WITH_AUDIO=0|1`。

## 飞牛 fnOS 打包

```bash
./scripts/pack-fnos.sh
```

依赖：`pnpm`、`go`、`fnpack`、`gcc`（CGO）。会拉取 Linux amd64 的 FFmpeg / ORT，构建前端与二进制，产出 `release/dist/FrameFlow_*.fpk`。

在 macOS 上交叉编译需 Linux C 编译器，例如：

```bash
brew install filosottile/musl-cross/musl-cross
CC=x86_64-linux-musl-gcc ./scripts/pack-fnos.sh
```

详见 [release/README.md](../release/README.md)。

## 其它

| 脚本 | 说明 |
|------|------|
| `build.sh` / `build.cmd` | 多平台无 CGO 打包（历史 go2rtc 风格；本仓库主路径为带 CGO 的本地构建 / `pack-fnos.sh`） |

更多库目录说明见 [lib/onnxruntime/README.md](../lib/onnxruntime/README.md)、[lib/ffmpeg/README.md](../lib/ffmpeg/README.md)。
