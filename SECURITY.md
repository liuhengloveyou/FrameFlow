# Security Policy

**English** | [简体中文](SECURITY.zh-CN.md)

## Supported versions

Security fixes are applied on the latest `main` branch and recent tagged releases when practical.

| Version | Supported |
|---------|-----------|
| `main`  | Yes       |
| latest release tag | Yes |
| older releases | Best effort |

## Reporting a vulnerability

Please **do not** open a public Issue for security vulnerabilities.

Prefer one of:

1. [GitHub Security Advisories](https://github.com/liuhengloveyou/FrameFlow/security/advisories/new) (once enabled on the repository)
2. Contact the maintainer via GitHub: [@liuhengloveyou](https://github.com/liuhengloveyou)

Include:

- Affected version / commit
- Impact description
- Reproduction steps or proof-of-concept (private)
- Whether you plan a fix PR

You should receive an acknowledgement within a reasonable time. Please allow time for assessment and coordinated disclosure before public discussion.

## Deployment notes

- Change the default `session_key` in `passport.conf.yaml` before production use
- Do not expose management ports to untrusted networks without authentication and TLS termination
- Keep dependencies and optional native libraries (FFmpeg / ONNX Runtime) updated
