package api

import (
	"net/http"
	"strings"
)

func middlewareLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Trace().Msgf("[api] %s %s %s", r.Method, r.URL, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func isLoopback(remoteAddr string) bool {
	return strings.HasPrefix(remoteAddr, "127.") || strings.HasPrefix(remoteAddr, "[::1]") || remoteAddr == "@"
}

func isUserCenterPath(path string) bool {
	return path == "/usercenter" || strings.HasPrefix(path, "/usercenter/")
}

func middlewareAuth(username, password string, localAuth bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Passport 会话登录不受 api Basic Auth 拦截
		if isUserCenterPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if localAuth || !isLoopback(r.RemoteAddr) {
			user, pass, ok := r.BasicAuth()
			if !ok || user != username || pass != password {
				w.Header().Set("Www-Authenticate", `Basic realm="go2rtc"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

var corsAllowOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://127.0.0.1:5173": true,
	"http://localhost:27788": true,
	"http://127.0.0.1:27788": true,
	"http://localhost:8888":  true,
	"http://127.0.0.1:8888":  true,
}

func middlewareCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && corsAllowOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		} else if origin == "" {
			// 同源请求无 Origin
		} else {
			// 未知 Origin：开发兜底允许回显（仍带 credentials 需明确 origin）
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API, X-Requested-With")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
