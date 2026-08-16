# FrameFlow 前端

Vite + React 19 + Astryx + TanStack Router / Query。业务数据一律请求真实后端 API。

## 开发

```bash
pnpm install
pnpm dev
```

浏览器打开 `http://127.0.0.1:5173`。登录走后端 passport（需本地 FrameFlow 服务）。

环境变量：

| 变量 | 说明 |
|------|------|
| `VITE_DEV_API_TARGET` | 开发代理目标，默认 `http://127.0.0.1:8888` |

```bash
# 查 Astryx 组件 API
pnpm astryx component Button
```

## 构建

```bash
pnpm build
```

产物在 `dist/`，由 Go 通过 [`embed.go`](embed.go) 嵌入；开发期也可配置：

```yaml
api:
  static_dir: frontend/dist
```

Go 服务会对非文件路径回退 `index.html`（SPA history）。

## 目录

- `src/routes` — TanStack 文件路由（薄）
- `src/features` — 业务页面
- `src/apis` — API 客户端（真实后端）
- `src/components` — 壳与通用 UI
