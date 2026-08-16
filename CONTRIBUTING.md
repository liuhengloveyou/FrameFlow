# Contributing

**English** | [简体中文](CONTRIBUTING.zh-CN.md)

Thanks for your interest in FrameFlow (巡帧). This guide explains how to contribute.

## Development environment

- Go **1.26.2+**
- Node.js LTS + pnpm
- Optional: `ffmpeg`, ONNX Runtime (vision-related features)

```bash
cd frontend && pnpm install && pnpm build && cd ..
go build -o frameflow .
go test ./...
```

Fetch native libraries only when needed: `./scripts/fetch-*.sh` and `./scripts/prepare-dev-libs.sh`. Do not commit prebuilt `.so` / `.dylib` or `.onnx` files.

## Branches and commits

1. Branch from latest `main`: `feature/…`, `fix/…`, or `docs/…`
2. Keep changes small and complete; do not commit `data/`, `docs/`, `参考/`, local secrets, or customer data
3. Prefer commit messages that explain **why** (English or Chinese)
4. Open a Pull Request and fill in the template

## Pull requests

- Describe motivation, behavior changes, and how you tested
- Call out breaking API / config changes
- CI (`.github/workflows/ci.yml`) should pass
- One PR, one concern; discuss large refactors in an Issue first

## Code style

- Go: match surrounding style; add brief comments for new public APIs
- Frontend: follow existing React / Astryx / TanStack structure; avoid drive-by formatting
- Do not add unused dependencies; do not commit `node_modules`

## Issues

- Bugs: repro steps, version (`./frameflow -v`), OS, relevant logs (redacted)
- Features: scenario and expected behavior; note if you plan to implement it

## License

Contributions are licensed under the project [MIT License](LICENSE).
