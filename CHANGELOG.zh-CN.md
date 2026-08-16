# Changelog

[English](CHANGELOG.md) | **简体中文**

本文件记录 FrameFlow（巡帧）面向发布的变更，格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

打包发版前请先完成本文件与英文版的 `[Unreleased]` → 正式版本迁移，再同步改版本号（见下方「发版改版本」）。

## 发版改版本

| 位置 | 字段 | 说明 |
|------|------|------|
| `CHANGELOG.md` / `CHANGELOG.zh-CN.md` | `## [x.y.z] - YYYY-MM-DD` | 把 Unreleased 落成正式条目 |
| `release/FrameFlow.fnos/manifest` | `version` | **fnOS `.fpk` 包版本**（`pack-fnos.sh` 用它命名产物） |
| `main.go` | `app.Version` | 二进制 `-v` / 日志里的版本 |
| `frontend/package.json` | `version` | 前端包版本（建议与上述对齐） |

建议流程：

1. 在 `[Unreleased]` 下按 **新增 / 变更 / 修复 / 移除** 写好本轮改动（中英文两份保持同步）。
2. 确定下一版本号（破坏性 → major，功能 → minor，修复 → patch）。
3. 把 Unreleased 内容改成 `## [x.y.z] - YYYY-MM-DD`，并清空 Unreleased 小节（保留标题）。
4. 同步改上表三处版本号（通常与 changelog 一致）。
5. 执行 `./scripts/pack-fnos.sh`，产物为 `release/dist/FrameFlow_x86_v{version}.fpk`。

---

## [Unreleased]

### 新增

- 开源仓库基建：MIT LICENSE、NOTICE、CONTRIBUTING、CODE_OF_CONDUCT、SECURITY、Issue/PR 模板与 CI
- RTSP 设备推流（push）接入：平台侧收流、推流密钥与展示 URL
- RTSP / RTMP / GB28181 表单统一「流传输模式」（TCP / UDP）
- RTSP 系统设置与设备页推流主机/端口联动

### 变更

- 设备协议选项移除 HLS（新增摄像头不再可选 HLS）
- `RtspTransport` 重命名为 `StreamTransport`（旧别名保留但已弃用）
- Docker 镜像与文档统一为 `frameflow` 命名（不再使用 go2rtc 产物名）

### 修复

- （打包前按实际测试结果补充）

---

## [1.0.0] - 2026-07-20

### 新增

- fnOS 普通应用打包（`.fpk`）：`scripts/pack-fnos.sh` + `release/FrameFlow.fnos`
- 内置 FFmpeg / ONNX Runtime 原生库随包分发
- Passport 认证与 RBAC
- 多协议接入：RTSP、RTMP、ONVIF、GB28181 等
- Web 控制台：设备管理、实时预览、AI 配置入口
- YOLO / FFmpeg 视觉推理链路
- WebRTC 预览与 GB28181 国标对接

[Unreleased]: https://github.com/liuhengloveyou/FrameFlow/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/liuhengloveyou/FrameFlow/releases/tag/v1.0.0
