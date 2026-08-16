package frontend

import (
	"embed"
	"io/fs"
)

// Dist 嵌入前端构建产物。开发期可用 api.static_dir: frontend/dist 覆盖。
//
//go:embed all:dist
var Dist embed.FS

// HasDist 判断嵌入的 dist 是否包含可用的 index.html。
func HasDist() bool {
	sub, err := fs.Sub(Dist, "dist")
	if err != nil {
		return false
	}
	f, err := sub.Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
