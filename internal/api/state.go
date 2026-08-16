package api

import (
	"net/http"

	"github.com/rs/zerolog"
)

// 进程级 HTTP 入口与配置（由 Init 写入，供 Listen 与各子模块 HandleFunc 使用）。
var (
	allowPaths []string
	basePath   string
	log        zerolog.Logger
)

var Handler http.Handler

var Port int
