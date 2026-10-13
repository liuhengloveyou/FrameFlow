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

当前钉版本 **1.30.0**（与 go.mod 的 `github.com/yalue/onnxruntime_go v1.30.0` 对齐；
onnxruntime_go 通过 OrtApi 函数表调用，minor 版本不匹配也能跑，但建议同步升级）。

**版本升级时**：改两个 fetch 脚本里的 `VER=`，删掉 `lib/onnxruntime/<os>/<arch>/`
旧目录，重跑 fetch + `./scripts/prepare-dev-libs.sh`。

## 执行提供器（EP）选择

推理加速后端由 `internal/vision/preprocess.go` 的 `applyBestExecutionProvider` 按平台自动选择：

| 平台 | 默认 EP | 说明 |
|---|---|---|
| darwin | CoreML | Apple GPU/ANE；yolo26l@640 实测 352ms → 34ms（10.3x） |
| linux/windows + NVIDIA GPU | CUDA | **需要 onnxruntime-gpu 版库**（CPU 包里没有 CUDA EP，追加失败会自动回退 CPU 并打警告） |
| 其他 | CPU | 池化多 session 并行 |

`FRAMEFLOW_YOLO_EP=cpu|coreml|cuda|dml|openvino|tensorrt` 可强制指定或禁用加速。

打包（Linux / 飞牛）时拷到 `fnos/FrameFlow/app/server/lib/`，启动脚本设置
`ONNXRUNTIME_SHARED_LIBRARY_PATH` / `LD_LIBRARY_PATH`。
本地开发也可设 `vision.ort_lib: lib/libonnxruntime.dylib`（Linux 为 `.so`）。
