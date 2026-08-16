# Docker

FrameFlow 提供 Dockerfile 以便自建镜像。公开多架构镜像仓库发布后续补充；当前请本地构建。

## 构建

在仓库根目录：

```bash
# 先构建前端（镜像构建阶段也可在 Dockerfile 中扩展；当前基础镜像示例以二进制为主）
cd frontend && pnpm install && pnpm build && cd ..

docker build -f docker/Dockerfile -t frameflow:local .
```

硬件加速 / Rockchip 变体见同目录：

- `docker/hardware.Dockerfile`
- `docker/rockchip.Dockerfile`

## Compose 示例

```yaml
services:
  frameflow:
    image: frameflow:local
    network_mode: host       # WebRTC / UDP 摄像头常用
    privileged: true         # 仅在需要 FFmpeg 硬件转码时
    restart: unless-stopped
    environment:
      - TZ=Asia/Shanghai
    volumes:
      - "~/frameflow-config:/config"   # 放置 frameflow.yaml
```

## 运行

```bash
docker run -d \
  --name frameflow \
  --network host \
  --restart unless-stopped \
  -e TZ=Asia/Shanghai \
  -v ~/frameflow-config:/config \
  frameflow:local
```

将配置文件挂载到容器工作目录（或镜像入口约定的路径），确保 `api.listen` 等端口可达。默认 Web UI：`http://127.0.0.1:27788`。

## 说明

- 镜像内通常依赖系统 `ffmpeg`；与仓库 `scripts/fetch-ffmpeg-*.sh` 的 GPL 预编译库是两条路径，分发时请阅读根目录 [NOTICE](../NOTICE)。
- 生产环境请更换 Passport `session_key`，并按需关闭 `privileged`。
