# FrameFlow

<p align="center">
  <img src="./assets/logo.png" alt="FrameFlow" width="160">
</p>

<p align="center">
  <strong>The Open-Source Infrastructure for Video AI</strong><br>
  Connect video streams, AI models, events, and workflows in one platform.
</p>

<p align="center">
  <a href="https://github.com/liuhengloveyou/FrameFlow/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/liuhengloveyou/FrameFlow/actions/workflows/ci.yml"><img src="https://github.com/liuhengloveyou/FrameFlow/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="./docker"><img src="https://img.shields.io/badge/Docker-self--build-2496ED?logo=docker&logoColor=white" alt="Docker"></a>
</p>

<p align="center">
  <b>English</b> · <a href="./README.zh-CN.md">简体中文</a>
  &nbsp;·&nbsp;
  <a href="https://github.com/liuhengloveyou/FrameFlow">GitHub</a>
  · <a href="./CONTRIBUTING.md">Contributing</a>
  · <a href="./docker">Docker</a>
</p>

---

FrameFlow is an open-source platform for building **AI-powered video applications**.

It connects **video streams**, **AI runtimes**, **events**, and **workflows** into one unified system — so you can ship video AI products without reinventing the infrastructure every time.

> Product UI screenshots / demo GIF will be added here soon.
> Until then, the architecture and positioning below are the source of truth.

## Why FrameFlow?

Traditional video systems are good at **watching** and **recording**.

AI video applications need much more:

- Connect cameras and video streams
- Run AI models on video
- Detect and understand events
- Trigger actions automatically
- Build business workflows
- Manage recordings and evidence
- Connect video intelligence with external systems

FrameFlow is designed as the **infrastructure layer** between video sources, AI models, and real-world applications:

```text
                 ┌─────────────────────┐
                 │    Video Sources    │
                 │ RTSP · GB28181 · …  │
                 └──────────┬──────────┘
                            │
                            ▼
                 ┌─────────────────────┐
                 │      FrameFlow      │
                 │                     │
                 │  Stream Management  │
                 │  AI Runtime         │
                 │  Event Center       │
                 │  Workflow Engine    │
                 │  Recording          │
                 └──────────┬──────────┘
                            │
              ┌─────────────┼─────────────┐
              ▼             ▼             ▼
          AI Events     Workflows     Applications
```

### Where FrameFlow sits in the ecosystem

FrameFlow does **not** try to replace every media server or NVR.

Think of the stack like this:

```text
Video Infrastructure
        │
        ├── MediaMTX
        ├── ZLMediaKit
        └── go2rtc  (streaming DNA FrameFlow builds on)
                │
                ▼
        ┌─────────────────┐
        │    FrameFlow    │
        │                 │
        │ AI Runtime      │
        │ Event Center    │
        │ Workflow Engine │
        │ Recording       │
        │ Application UI  │
        └─────────────────┘
                │
                ▼
        Video AI Applications
```

| Project type | Typical strength | Gap FrameFlow focuses on |
|--------------|------------------|--------------------------|
| **MediaMTX / ZLMediaKit / go2rtc** | Reliable stream ingest & fan-out | Application-level AI events & workflows |
| **Frigate / Shinobi / ZoneMinder** | Home / SMB NVR + detection UX | Open platform to bring your own models, events, and business workflows |
| **Traditional NVR / VMS** | Recording, live view, ops UI | Developer-first Video → AI → Event → Workflow → Application path |

FrameFlow’s thesis:

**Video → AI → Event → Workflow → Application**

## What Can You Build?

FrameFlow is not just a video player or an NVR. It is for applications such as:

- Industrial safety monitoring
- Construction site monitoring
- Retail analytics
- Campus safety
- Traffic and parking analysis
- Smart security
- Pet and home monitoring
- Warehouse intelligence
- Enterprise video intelligence

Example path:

```text
Camera
   │
   ▼
Person Detection
   │
   ▼
Person enters restricted area
   │
   ▼
Event Center
   │
   ▼
Workflow
   ├── Save snapshot
   ├── Start recording
   ├── Send notification
   └── Call external API
```

Goal: turn *“something happened in a video”* into *“FrameFlow understood it → the system automatically did something.”*

## Core Features

### Video Sources

Connect and manage streams from multiple sources:

- RTSP / RTMP
- GB28181
- WebRTC preview
- IP cameras & NVRs
- Other gateways (extensible modules)

### AI Runtime

Run models against video without hard-wiring your app to one AI framework.

Designed for workloads such as:

- Object / person / face detection
- OCR, pose, classification
- Custom ONNX models — **bring your own model**

### Event Center

Turn AI outputs into structured events, rules, and actions:

```text
AI Result → Event → Rules → Actions
```

Examples: intrusion, fire, PPE violation, abnormal behavior.

### Workflow Engine

Wire events to reusable actions (snapshot, record, notify, webhook) instead of one-off integration glue.

### Recording & Evidence

Event-driven recording, playback, snapshots, and evidence retrieval around AI incidents.

### Spatial Management

Organize cameras by real-world places: organization → campus / factory → building → floor → camera.

## Architecture

FrameFlow separates video infrastructure, AI inference, events, and business workflows:

```text
┌──────────────────────────────────────────────────────┐
│                    Applications                      │
│  Dashboard · Monitoring · Security · Industry Apps   │
└──────────────────────────┬───────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────┐
│                     FrameFlow                        │
│  Camera · Stream · Event · Workflow · Recording · Map│
└───────────────┬──────────────────────┬───────────────┘
                │                      │
                ▼                      ▼
       ┌────────────────┐     ┌─────────────────┐
       │   AI Runtime   │     │ Video Runtime   │
       │ Detection/OCR… │     │ RTSP/GB/WebRTC  │
       └────────────────┘     └─────────────────┘
```

Components are intentionally modular so you can replace or extend pieces without rewriting the whole stack.

## Quick Start

### Requirements

- Go **1.26.2+**
- Node.js + [pnpm](https://pnpm.io/) (Web console)
- Recommended: system `ffmpeg`
- Optional: ONNX Runtime + YOLO `.onnx` for vision

### Build & run

```bash
git clone https://github.com/liuhengloveyou/FrameFlow.git
cd FrameFlow

cd frontend && pnpm install && pnpm build && cd ..
go build -o frameflow .

./frameflow -c configs/frameflow.yaml
```

Open **http://127.0.0.1:27788**

| Service | Default |
|---------|---------|
| HTTP / Web UI / API | `:27788` |
| RTSP | `:8554` |
| RTMP | `:1935` |
| WebRTC | `:8555` |

Docker self-build: see [docker/README.md](docker/README.md). Official compose images will be published later.

Native libs (optional vision / CGO packaging):

```bash
./scripts/fetch-onnxruntime-linux.sh   # or fetch-onnxruntime-osx.sh
./scripts/prepare-dev-libs.sh
```

This repo does **not** ship `.onnx` weights or prebuilt `.so` / `.dylib` binaries.

## Project Structure

```text
FrameFlow/
├── frontend/          # Web console (Vite + React), embedded via go:embed
├── internal/          # App modules: streams, cameras, vision, gb28181, workflows…
├── pkg/               # Shared media / protocol libraries
├── configs/           # Default YAML templates
├── docker/            # Dockerfiles for self-build
├── scripts/           # fetch native libs, packaging helpers
├── assets/            # README / brand assets
├── www/               # Legacy static UI fallback
└── release/           # fnOS packaging metadata
```

Layout may evolve as the platform grows.

## Roadmap

FrameFlow is under active development. Priorities may change with community feedback.

**Core platform** — camera & preview, multi-source streams, recording, event center, workflow engine

**AI** — AI Runtime, model store, multiple backends, custom models, GPU scheduling

**Video** — RTSP / GB28181 / WebRTC depth, hardware acceleration, distributed processing

**Enterprise** — multi-user, RBAC, multi-tenant, audit logs, distributed deploy

## Who Is FrameFlow For?

- Teams building AI camera / video analytics products
- Integrators adding AI into existing VMS/NVR stacks
- Industrial vision and safety applications
- Developers who want an open foundation for Video AI instead of gluing every piece from scratch

## Why Open Source?

Video AI stacks are fragmented: camera + gateway + model + queue + events + workflow + storage + UI.

Rebuilding that glue for every product is expensive. FrameFlow aims to be an **open foundation** so you spend time on intelligence and applications — not on reinventing video infrastructure.

## Contributing

Contributions are welcome: bugs, docs, integrations, AI Runtime adapters, workflow nodes, video pipelines, and PRs.

Please read [CONTRIBUTING.md](CONTRIBUTING.md) ([简体中文](CONTRIBUTING.zh-CN.md)) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

Security reports: [SECURITY.md](SECURITY.md) ([简体中文](SECURITY.zh-CN.md)) — privately, not in public issues.

## Community

- [GitHub Issues](https://github.com/liuhengloveyou/FrameFlow/issues)
- [Discussions](https://github.com/liuhengloveyou/FrameFlow/discussions) (enable after public launch)
- Pull Requests

More channels will appear as the project grows.

## License

FrameFlow source is released under the [MIT License](LICENSE).

Third-party notices (including go2rtc heritage and optional GPL FFmpeg prebuilds): [NOTICE](NOTICE).

---

<p align="center">
  <strong>FrameFlow</strong><br>
  Build the infrastructure. Ship the intelligence.
</p>
