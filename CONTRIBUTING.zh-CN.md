# 贡献指南

[English](CONTRIBUTING.md) | **简体中文**

感谢你对 FrameFlow（巡帧）的兴趣。本文说明如何参与开发。

## 开发环境

- Go **1.26.2+**
- Node.js LTS + pnpm
- 可选：`ffmpeg`、ONNX Runtime（视觉相关功能）

```bash
cd frontend && pnpm install && pnpm build && cd ..
go build -o frameflow .
go test ./...
```

需要原生库时再执行 `./scripts/fetch-*.sh` 与 `./scripts/prepare-dev-libs.sh`。仓库不提交预编译 `.so` / `.dylib` 与 `.onnx`。

## 分支与提交

1. 从最新 `main` 开分支：`feature/…`、`fix/…` 或 `docs/…`
2. 改动尽量小而完整；不要提交 `data/`、`docs/`、`参考/`、本地密钥或客户数据
3. 提交信息简明说明**原因**（中英文均可）
4. 推送后发起 Pull Request，按模板填写说明

## Pull Request

- 说明动机、行为变化、测试方式
- 涉及 API / 配置时注明破坏性变更
- CI（`.github/workflows/ci.yml`）应通过
- 一个 PR 聚焦一件事；大重构请先开 Issue 讨论

## 代码约定

- Go：与周边文件风格一致；新增公开 API 时补充简短注释
- 前端：遵循现有 React / Astryx / TanStack 结构；勿用无关格式化刷屏
- 不要引入未使用的依赖；不要提交 `node_modules`

## Issue

- Bug：复现步骤、版本（`./frameflow -v`）、OS、相关日志（脱敏）
- 功能：场景与期望行为；是否愿意自行实现可注明

## 许可证

贡献默认按项目 [MIT License](LICENSE) 授权。
