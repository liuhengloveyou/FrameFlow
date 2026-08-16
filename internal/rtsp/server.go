package rtsp

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/internal/app"
	"github.com/liuhengloveyou/FrameFlow/pkg/rtsp"
)

// 运行时鉴权 / 默认媒体（HTTP 更新后立即用于新连接）。
type runtimeAuth struct {
	username   string
	password   string
	packetSize uint16
}

var (
	serverMu sync.Mutex
	serverLn net.Listener
	authMu   sync.RWMutex
	rtAuth   runtimeAuth
)

func setRuntimeFromRow(row app.RTSPConfig) {
	dq := strings.TrimSpace(row.DefaultQuery)
	if dq == "" {
		dq = "video&audio"
	}
	if query, err := url.ParseQuery(dq); err == nil {
		defaultMedias = ParseQuery(query)
	}
	authMu.Lock()
	rtAuth = runtimeAuth{
		username:   strings.TrimSpace(row.Username),
		password:   row.Password,
		packetSize: uint16(row.PacketSize),
	}
	authMu.Unlock()
}

func getRuntimeAuth() runtimeAuth {
	authMu.RLock()
	defer authMu.RUnlock()
	return rtAuth
}

// applyListenState 按库内 enabled 启停；并刷新运行时鉴权/默认媒体。
func applyListenState(row app.RTSPConfig) error {
	setRuntimeFromRow(row)
	listen := row.ListenAddr()

	serverMu.Lock()
	defer serverMu.Unlock()

	if listen == "" {
		return stopListenLocked()
	}
	if serverLn != nil && sameListenPort(serverLn.Addr().String(), listen) {
		return nil
	}
	if err := stopListenLocked(); err != nil {
		return err
	}
	return startListenLocked(listen)
}

func startListenLocked(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	serverLn = ln
	log.Info().Str("addr", addr).Msg("[rtsp] listen started")
	go acceptLoop(ln)
	return nil
}

func stopListenLocked() error {
	if serverLn == nil {
		return nil
	}
	err := serverLn.Close()
	serverLn = nil
	log.Info().Msg("[rtsp] listen stopped")
	return err
}

func acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go serveConn(conn)
	}
}

func serveConn(conn net.Conn) {
	c := rtsp.NewServer(conn)
	auth := getRuntimeAuth()
	c.PacketSize = auth.packetSize
	if auth.username != "" {
		if ta, ok := conn.RemoteAddr().(*net.TCPAddr); ok && !ta.IP.IsLoopback() {
			c.Auth(auth.username, auth.password)
		}
	}
	tcpHandler(c)
}

func sameListenPort(a, b string) bool {
	return listenPortOf(a) > 0 && listenPortOf(a) == listenPortOf(b)
}

func listenPortOf(addr string) int {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return 0
	}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		p, _ := strconv.Atoi(port)
		return p
	}
	if strings.HasPrefix(addr, ":") {
		p, _ := strconv.Atoi(strings.TrimPrefix(addr, ":"))
		return p
	}
	return 0
}
