# Changelog

**English** | [简体中文](CHANGELOG.zh-CN.md)

All notable changes to FrameFlow (巡帧) are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning follows [Semantic Versioning](https://semver.org/).

Before a release, move `[Unreleased]` into a dated version section and bump version fields (see “Release checklist”).

## Release checklist

| Location | Field | Notes |
|----------|-------|-------|
| `CHANGELOG.md` / `CHANGELOG.zh-CN.md` | `## [x.y.z] - YYYY-MM-DD` | Promote Unreleased into a release entry |
| `release/FrameFlow.fnos/manifest` | `version` | **fnOS `.fpk` package version** (used by `pack-fnos.sh`) |
| `main.go` | `app.Version` | Binary `-v` / log version |
| `frontend/package.json` | `version` | Frontend package version (keep aligned) |

Suggested flow:

1. Record changes under `[Unreleased]` as **Added / Changed / Fixed / Removed**.
2. Choose the next semver (breaking → major, feature → minor, fix → patch).
3. Rename Unreleased content to `## [x.y.z] - YYYY-MM-DD` and clear the Unreleased section (keep the heading).
4. Sync the three version fields above (usually the same as the changelog).
5. Run `./scripts/pack-fnos.sh` → `release/dist/FrameFlow_x86_v{version}.fpk`.

---

## [Unreleased]

### Added

- Open-source project scaffolding: MIT LICENSE, NOTICE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, Issue/PR templates, and CI
- RTSP device push ingest: platform receive path, push key, and display URL
- Unified stream transport mode (TCP / UDP) across RTSP / RTMP / GB28181 forms
- RTSP system settings linked with device-page push host/port

### Changed

- Removed HLS from device protocol options (new cameras can no longer select HLS)
- Renamed `RtspTransport` to `StreamTransport` (legacy alias kept, deprecated)
- Docker images and docs unified under the `frameflow` name (no longer go2rtc artifact names)

### Fixed

- (Fill in after pre-release testing)

---

## [1.0.0] - 2026-07-20

### Added

- fnOS app packaging (`.fpk`): `scripts/pack-fnos.sh` + `release/FrameFlow.fnos`
- Bundled FFmpeg / ONNX Runtime native libs for packaging
- Passport authentication and RBAC
- Multi-protocol ingest: RTSP, RTMP, ONVIF, GB28181, and more
- Web console: devices, live preview, AI config entry points
- YOLO / FFmpeg vision inference path
- WebRTC preview and GB28181 national-standard integration

[Unreleased]: https://github.com/liuhengloveyou/FrameFlow/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/liuhengloveyou/FrameFlow/releases/tag/v1.0.0
