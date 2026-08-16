# 安全策略

[English](SECURITY.md) | **简体中文**

## 支持的版本

安全修复优先合入最新 `main`，并在可行时回补近期打 tag 的发行版。

| 版本 | 是否支持 |
|------|----------|
| `main` | 是 |
| 最新 release tag | 是 |
| 更早发行版 | 尽力而为 |

## 报告漏洞

请**不要**在公开 Issue 中报告安全漏洞。

优先使用：

1. [GitHub Security Advisories](https://github.com/liuhengloveyou/FrameFlow/security/advisories/new)（仓库启用后）
2. 通过 GitHub 联系维护者：[@liuhengloveyou](https://github.com/liuhengloveyou)

请尽量包含：

- 受影响版本 / commit
- 影响说明
- 复现步骤或 PoC（私下提供）
- 是否计划提交修复 PR

我们会在合理时间内确认收到。请在评估与协调披露完成前，勿公开讨论细节。

## 部署提示

- 生产环境务必更换 `passport.conf.yaml` 中的默认 `session_key`
- 未做认证与 TLS 终结前，勿将管理端口暴露给不可信网络
- 保持依赖及可选原生库（FFmpeg / ONNX Runtime）更新
