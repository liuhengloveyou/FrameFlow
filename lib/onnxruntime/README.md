# 预编译 ONNX Runtime

供 vision YOLO 推理加载（运行时入口：`lib/libonnxruntime.{so|dylib}`，由 `prepare-dev-libs.sh` 链接）。

## 目录

```
lib/onnxruntime/linux/amd64/libonnxruntime.so*
lib/onnxruntime/linux/arm64/libonnxruntime.so*
lib/onnxruntime/darwin/arm64/libonnxruntime*.dylib
lib/onnxruntime/darwin/amd64/libonnxruntime*.dylib
```

## 获取

```bash
# Linux
./scripts/fetch-onnxruntime-linux.sh        # amd64
./scripts/fetch-onnxruntime-linux.sh arm64

# macOS
./scripts/fetch-onnxruntime-osx.sh          # arm64
./scripts/fetch-onnxruntime-osx.sh amd64

# 链到 lib/libonnxruntime.*
./scripts/prepare-dev-libs.sh
```

当前钉版本 **1.26.0**。

打包（Linux / 飞牛）时拷到 `fnos/FrameFlow/app/server/lib/`，启动脚本设置
`ONNXRUNTIME_SHARED_LIBRARY_PATH` / `LD_LIBRARY_PATH`。
本地开发也可设 `vision.ort_lib: lib/libonnxruntime.dylib`（Linux 为 `.so`）。
