# ONVIF

## ONVIF Client

[`new in v1.5.0`](https://github.com/liuhengloveyou/FrameFlow/releases/tag/v1.5.0)

The source is not very useful if you already know RTSP and snapshot links for your camera. But it can be useful if you don't.

**设备管理**添加 ONVIF 设备（与 RTSP 拉流类似：IP/端口 + 更多选项里的账号 / Profile）。

**系统 → 协议配置 → ONVIF**：服务端开关、发现超时、发现默认账号。

```yaml
streams:
  dahua1: onvif://admin:password@192.168.1.123
  reolink1: onvif://admin:password@192.168.1.123:8000?subtype=0
  tapo1: onvif://admin:password@192.168.1.123:2020?subtype=1
```

`subtype` 为 **0-based** Profile 序号（或 Profile token）。设备表单「Profile 序号」为 **1-based**（1=主码流）。

HTTP API：

- `GET /api/onvif` — 局域网发现
- `GET /api/onvif?src=onvif://user:pass@host:port` — 列出 Profiles
- `GET/PUT /api/onvif/settings` — 协议配置（对齐 RTSP/RTMP settings）

## ONVIF Server

A regular camera has a single video source (`GetVideoSources`) and two profiles (`GetProfiles`).

Go2rtc has one video source and one profile per stream.

## Tested clients

Go2rtc works as ONVIF server:

- Happytime onvif client (windows)
- Home Assistant ONVIF integration (linux)
- Onvier (android)
- ONVIF Device Manager (windows)

PS. Supports only TCP transport for RTSP protocol. UDP and HTTP transports - unsupported yet.

## Tested cameras

Go2rtc works as ONVIF client:

- Dahua IPC-K42
- OpenIPC
- Reolink RLC-520A
- TP-Link Tapo TC60
