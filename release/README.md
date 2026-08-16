# FrameFlow · fnOS 普通应用（非 Docker）

目录 `release/FrameFlow.fnos` 由 `fnpack create FrameFlow` 生成，已按 **原生二进制** 方式接好启停与 Web 入口。

## 发版前改版本

打包前先更新根目录 [`CHANGELOG.md`](../CHANGELOG.md)：

1. 把 `[Unreleased]` 写成正式版本条目（日期 + 变更说明）
2. 同步改版本号：
   - `release/FrameFlow.fnos/manifest` 的 `version`（决定 `.fpk` 文件名）
   - `main.go` 的 `app.Version`
   - `frontend/package.json` 的 `version`（建议一致）

## 一键打包

```bash
./scripts/pack-fnos.sh
```

产物：`release/dist/FrameFlow_x86_v*.fpk`。需要 `pnpm`、`go`、`fnpack`、`gcc` 在 PATH 中。  
`fnpack` 下载：https://static2.fnnas.com/fnpack/

打包脚本会自动：

1. 下载 FFmpeg → `lib/ffmpeg/linux/amd64/`
2. 下载 ONNX Runtime → `lib/onnxruntime/linux/amd64/`
3. 复制到 `app/server/lib/`，与二进制一起打进 `.fpk`
4. 用户机通过 `$ORIGIN/lib` + `LD_LIBRARY_PATH` / `ONNXRUNTIME_SHARED_LIBRARY_PATH` 加载，**无需安装系统开发库**

也可单独拉取 / 本地开发：

```bash
./scripts/fetch-ffmpeg-linux.sh          # 或手动解压到 lib/ffmpeg/linux/amd64/
./scripts/fetch-onnxruntime-linux.sh
./scripts/prepare-dev-libs.sh            # 链到 lib/libonnxruntime.*（linux/darwin 通用）
CGO_ENABLED=1 go build -o FrameFlow .
export ONNXRUNTIME_SHARED_LIBRARY_PATH="$PWD/lib/libonnxruntime.so"
./FrameFlow
```

> macOS 交叉编译到 linux 还需 Linux C 编译器，例如：  
> `brew install filosottile/musl-cross/musl-cross`，然后  
> `CC=x86_64-linux-musl-gcc ./scripts/pack-fnos.sh`  
> 在 Linux 上打包则无需设 `CC`。

## 目录说明

| 路径 | 作用 |
|------|------|
| `manifest` | 应用元数据，`service_port=27788` |
| `app/ui/config` | 桌面入口标题「巡帧」，打开 `http://<nas>:27788/` |
| `app/server/FrameFlow` | 由打包脚本写入的 linux 二进制 |
| `app/server/frameflow.yaml` | 默认配置模板，安装时复制到 `$TRIM_PKGETC` |
| `app/server/passport.conf.yaml` | passport 默认配置，安装到 `$TRIM_PKGETC`，启动 `-passport` 指定 |
| `app/server/rbac_with_domains_model.conf` | Casbin 模型，安装到 `$TRIM_PKGETC`，并软链到工作目录 |
| `cmd/main` | start / stop / status |
| `cmd/install_callback` | 安装后初始化配置与数据目录 |

运行时：

- 配置：`$TRIM_PKGETC/frameflow.yaml`、`passport.conf.yaml`、`rbac_with_domains_model.conf`
- 数据/DB：`$TRIM_PKGVAR/data/`
- 日志：`$TRIM_PKGVAR/info.log`

## 安装测试

1. 复制 `.fpk` 到飞牛测试机
2. 应用中心 → 本地安装
3. 启动应用，浏览器打开管理口 `27788`

## 注意

- `manifest.platform=x86` → 脚本编译 **linux/amd64**；ARM 需改 manifest 并改脚本里的 `GOARCH`
- WebRTC / GB28181 / RTSP 等端口需在 NAS 防火墙放行
- ONNX 推理若需要，把 `libonnxruntime.so` 放入数据目录，并配置 `vision.ort_lib`
- 图标可替换根目录 `ICON.PNG` / `ICON_256.PNG` 与 `app/ui/images/`
