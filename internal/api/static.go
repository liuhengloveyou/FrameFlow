package api

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/frontend"
	"github.com/liuhengloveyou/FrameFlow/www"
)

func initStatic(staticDir string) {
	var root http.FileSystem
	var spaIndex []byte

	if staticDir != "" {
		log.Info().Str("dir", staticDir).Msg("[api] serve static")
		root = http.Dir(staticDir)
		if b, err := os.ReadFile(path.Join(staticDir, "index.html")); err == nil {
			spaIndex = b
		}
	} else if frontend.HasDist() {
		log.Info().Msg("[api] serve embedded frontend dist")
		sub, err := fs.Sub(frontend.Dist, "dist")
		if err != nil {
			log.Error().Err(err).Msg("[api] frontend dist subfs")
			root = http.FS(www.Static)
		} else {
			root = http.FS(sub)
			spaIndex, _ = fs.ReadFile(sub, "index.html")
		}
	} else {
		root = http.FS(www.Static)
	}

	base := len(basePath)
	fileServer := http.FileServer(root)

	RegisterBizAPI("", "", false, "", func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path
		if base > 0 {
			reqPath = reqPath[base:]
			r.URL.Path = reqPath
		}

		// SPA fallback：非 API、非真实静态资源时回退 index.html
		if len(spaIndex) > 0 && shouldSPAFallback(reqPath, root) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(spaIndex)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

func shouldSPAFallback(reqPath string, root http.FileSystem) bool {
	clean := path.Clean("/" + strings.TrimPrefix(reqPath, "/"))
	if clean == "/" || clean == "/index.html" {
		return false
	}
	if strings.HasPrefix(clean, "/api") {
		return false
	}
	// 带扩展名的静态资源不走 SPA
	if ext := path.Ext(clean); ext != "" && ext != ".html" {
		return false
	}
	f, err := root.Open(strings.TrimPrefix(clean, "/"))
	if err == nil {
		_ = f.Close()
		return false
	}
	return true
}
