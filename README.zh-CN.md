# FrameFlow（巡帧）

<p align="center">
  <img src="./assets/logo.png" alt="FrameFlow" width="160">
</p>

<p align="center">
  <strong>面向 Video AI 的开源基础设施</strong><br>
  将视频流、AI 模型、事件和业务工作流连接起来。
</p>

<p align="center">
  <a href="https://github.com/liuhengloveyou/FrameFlow/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/liuhengloveyou/FrameFlow/actions/workflows/ci.yml"><img src="https://github.com/liuhengloveyou/FrameFlow/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="./docker"><img src="https://img.shields.io/badge/Docker-self--build-2496ED?logo=docker&logoColor=white" alt="Docker"></a>
</p>

<p align="center">
  <a href="./README.md">English</a> · <b>简体中文</b>
  &nbsp;·&nbsp;
  <a href="https://github.com/liuhengloveyou/FrameFlow">GitHub</a>
  · <a href="./CONTRIBUTING.zh-CN.md">贡献</a>
  · <a href="./docker">Docker</a>
</p>

---

FrameFlow 是一个面向 **AI 视频应用** 的开源平台。

它把 **视频流、AI Runtime、事件中心、工作流** 连成统一系统，让你不必每次从零拼装视频基础设施，就能交付 Video AI 产品。

> 产品界面截图 / Demo GIF 即将补充。在此之前，以下定位与架构是理解 FrameFlow 的主入口。

## 为什么是 FrameFlow？

传统视频系统擅长 **看** 和 **录**。

AI 视频应用需要更多：

- 接入摄像头与视频流
- 在视频上跑 AI 模型
- 识别并理解事件
- 自动触发动作
- 编排业务工作流
- 管理录像与证据
- 把视频智能接到外部系统

FrameFlow 要做的是：视频源、AI 模型与真实业务之间的 **基础设施层**：

```text
                 ┌─────────────────────┐
                 │       视频源        │
                 │ RTSP · GB28181 · …  │
                 └──────────┬──────────┘
                            │
                            ▼
                 ┌─────────────────────┐
                 │      FrameFlow      │
                 │                     │
                 │  流管理             │
                 │  AI Runtime         │
                 │  事件中心           │
                 │  工作流引擎         │
                 │  录像               │
                 └──────────┬──────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
           AI 事件        工作流         应用
```

### 在生态中的位置

FrameFlow **不是**要取代所有流媒体服务器或 NVR，而是补齐它们之上的一层：

```text
视频基础设施
        │
        ├── MediaMTX
        ├── ZLMediaKit
        └── go2rtc  （FrameFlow 流媒体能力的重要基因）
                │
                ▼
        ┌─────────────────┐
        │    FrameFlow    │
        │                 │
        │ AI Runtime      │
        │ 事件中心        │
        │ 工作流引擎      │
        │ 录像            │
        │ 应用控制台      │
        └─────────────────┘
                │
                ▼
        Video AI 应用
```

| 类型 | 常见长处 | FrameFlow 侧重补齐的缺口 |
|------|----------|--------------------------|
| **MediaMTX / ZLMediaKit / go2rtc** | 稳定的收流与分发 | 应用级 AI 事件与工作流 |
| **Frigate / Shinobi / ZoneMinder** | 家用/中小场景 NVR + 检测体验 | 开放平台：自带模型、事件与业务编排 |
| **传统 NVR / VMS** | 录像、预览、运维 UI | 开发者友好的 视频→AI→事件→工作流→应用 路径 |

FrameFlow 的主张：

**视频 → AI → 事件 → 工作流 → 应用**

## 能用来做什么？

FrameFlow 不只是播放器或 NVR，而是面向例如：

- 工业安全监控
- 工地监控
- 零售分析
- 校园安全
- 交通与停车分析
- 智慧安防
- 家庭 / 宠物看护
- 仓储智能
- 企业级视频智能

示例链路：

```text
摄像头
   │
   ▼
人员检测
   │
   ▼
人员进入禁区
   │
   ▼
事件中心
   │
   ▼
工作流
   ├── 保存截图
   ├── 开始录像
   ├── 发送通知
   └── 调用外部 API
```

目标：把「视频里发生了什么」变成「FrameFlow 理解了 → 系统自动做了什么」。

## 核心能力

### 视频源

多协议接入与管理：RTSP / RTMP、GB28181、WebRTC 预览、IPC / NVR，以及可扩展网关模块。

### AI Runtime

在视频上跑推理，而不把业务绑死在某一 AI 框架上。支持目标/人员/人脸检测、OCR、姿态、分类，以及自定义 ONNX——**Bring your own model**。

### 事件中心

把 AI 结果变成结构化事件、规则与动作：

```text
AI 结果 → 事件 → 规则 → 动作
```

例如：入侵、烟火、未戴 PPE、异常行为等。

### 工作流引擎

把事件接到可复用动作（截图、录像、通知、Webhook），而不是每个场景手写集成。

### 录像与证据

围绕 AI 事件做录像、回放、截图与证据检索。

### 空间管理

按真实空间组织摄像头：组织 → 园区/工厂 → 楼栋 → 楼层 → 设备。

## 架构

```text
┌──────────────────────────────────────────────────────┐
│                       应用层                         │
│     控制台 · 监控 · 安防 · 行业应用                   │
└──────────────────────────┬───────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────┐
│                     FrameFlow                        │
│  设备 · 流 · 事件 · 工作流 · 录像 · 地图             │
└───────────────┬──────────────────────┬───────────────┘
                │                      │
                ▼                      ▼
       ┌────────────────┐     ┌─────────────────┐
       │   AI Runtime   │     │   视频 Runtime  │
       │ 检测 / OCR …   │     │ RTSP/国标/WebRTC│
       └────────────────┘     └─────────────────┘
```

模块可替换、可扩展，不必推倒重来。

## 快速开始

### 依赖

- Go **1.26.2+**
- Node.js + [pnpm](https://pnpm.io/)
- 建议安装系统 `ffmpeg`
- 可选：ONNX Runtime + YOLO `.onnx`

### 构建与运行

```bash
git clone https://github.com/liuhengloveyou/FrameFlow.git
cd FrameFlow

cd frontend && pnpm install && pnpm build && cd ..
go build -o frameflow .

./frameflow -c configs/frameflow.yaml
```

浏览器打开 **http://127.0.0.1:27788**

| 服务 | 默认 |
|------|------|
| HTTP / Web UI / API | `:27788` |
| RTSP | `:8554` |
| RTMP | `:1935` |
| WebRTC | `:8555` |

Docker 自建见 [docker/README.md](docker/README.md)。官方 Compose 镜像后续发布。

可选原生库：

```bash
./scripts/fetch-onnxruntime-linux.sh   # 或 fetch-onnxruntime-osx.sh
./scripts/prepare-dev-libs.sh
```

仓库**不附带** `.onnx` 与预编译 `.so` / `.dylib`。

## 仓库结构

```text
FrameFlow/
├── frontend/          # Web 控制台（Vite + React），go:embed
├── internal/          # 业务模块：流、设备、视觉、国标、工作流…
├── pkg/               # 共享媒体 / 协议库
├── configs/           # 默认 YAML 模板
├── docker/            # 自建镜像 Dockerfile
├── scripts/           # 拉取原生库、打包脚本
├── assets/            # README / 品牌资源
├── www/               # 旧版静态页回退
└── release/           # fnOS 打包元数据
```

目录会随平台演进调整。

## 路线图

FrameFlow 仍在积极开发，优先级会随社区与真实场景调整。

**核心平台** — 设备与预览、多源流、录像、事件中心、工作流引擎

**AI** — AI Runtime、模型仓库、多推理后端、自定义模型、GPU 调度

**视频** — RTSP / GB28181 / WebRTC 加深、硬解加速、分布式处理

**企业** — 多用户、RBAC、多租户、审计、分布式部署

## 适合谁？

- 要做 AI 摄像头 / 视频分析产品的团队
- 想在现有 VMS/NVR 上接入 AI 的集成方
- 工业视觉与安全场景
- 希望有一套 Video AI 开源底座、而不是每次从零粘合组件的开发者

## 为什么开源？

Video AI 栈往往是碎片化的：摄像头 + 网关 + 模型 + 队列 + 事件 + 工作流 + 存储 + 前端。

为每个产品重造一遍成本很高。FrameFlow 希望提供开放底座，让你把时间花在智能与应用上，而不是反复重建视频基础设施。

## 贡献

欢迎提交 Bug、文档、集成、AI Runtime 适配、工作流节点、视频管线、示例与 PR。

请先阅读 [CONTRIBUTING.zh-CN.md](CONTRIBUTING.zh-CN.md)（[English](CONTRIBUTING.md)）与 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。

安全问题：[SECURITY.zh-CN.md](SECURITY.zh-CN.md)（[English](SECURITY.md)）——私下披露，勿贴公开 Issue。

## 社区

- [GitHub Issues](https://github.com/liuhengloveyou/FrameFlow/issues)
- [Discussions](https://github.com/liuhengloveyou/FrameFlow/discussions)（公开后启用）
- Pull Requests

渠道会随项目成长继续增加。

## 许可证

源码以 [MIT License](LICENSE) 发布。

第三方说明（含 go2rtc 渊源与可选 GPL FFmpeg 预编译库）：[NOTICE](NOTICE)。

---

<p align="center">
  <strong>FrameFlow</strong><br>
  Build the infrastructure. Ship the intelligence.
</p>
