# 预编译 FFmpeg（Linux）

供 `internal/ffdecode` CGO 链接，并随飞牛 `.fpk` 一起分发。

## 目录

```
lib/ffmpeg/linux/amd64/{include,lib}/
lib/ffmpeg/linux/arm64/{include,lib}/
```

（与 `lib/onnxruntime/{linux|darwin}/…` 同属运行/链接用原生依赖；根目录 `lib/libonnxruntime.*` 由 `prepare-dev-libs.sh` 生成。`参考/` 仅供对照实现，勿往里放构建依赖。）

## 获取

```bash
./scripts/fetch-ffmpeg-linux.sh        # amd64
./scripts/fetch-ffmpeg-linux.sh arm64
```

来源：[BtbN/FFmpeg-Builds](https://github.com/BtbN/FFmpeg-Builds) `n8.1-latest` gpl-shared。

国内下载慢可设镜像前缀：

```bash
export GITHUB_PROXY=https://ghfast.top/
./scripts/fetch-ffmpeg-linux.sh
```

## 运行时

打包脚本把 `lib/*.so*` 拷到 `fnos/FrameFlow/app/server/lib/`，
`cmd/main` 设置 `LD_LIBRARY_PATH`；二进制另有 `$ORIGIN/lib` rpath。
CGO 会强制保留对 `libswresample` 的直接依赖（避免 `--as-needed` 丢掉后，
传递依赖无法经 RUNPATH 解析）。

手搓验证时也可：

```bash
LD_LIBRARY_PATH=/vol2/@appcenter/FrameFlow/server/lib \
  /vol2/@appcenter/FrameFlow/server/FrameFlow -c /path/to/frameflow.yaml
```

macOS 本地开发仍可用 Homebrew FFmpeg（`pkg-config`）。
