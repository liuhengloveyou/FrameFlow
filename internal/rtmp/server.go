package rtmp

import (
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/internal/streams"
)

// 实际 TCP listener。期望状态一律读 app.RTMPConfig（库），不另存内存配置副本。
var (
	serverMu sync.Mutex
	serverLn net.Listener
)

// applyListenState 按 listen 启停：非空则绑定，空则停止。
func applyListenState(listen string, tokenQuery string) error {
	listen = strings.TrimSpace(listen)
	tq := strings.TrimSpace(tokenQuery)

	serverMu.Lock()
	defer serverMu.Unlock()

	if tq != "" {
		pushTokenQuery = tq
	} else {
		pushTokenQuery = defaultPushTokenQuery()
	}

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
	log.Info().Str("addr", addr).Msg("[rtmp] listen started")
	go acceptLoop(ln)
	return nil
}

func stopListenLocked() error {
	if serverLn == nil {
		return nil
	}
	err := serverLn.Close()
	serverLn = nil
	log.Info().Msg("[rtmp] listen stopped")
	return err
}

func acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			if err := tcpHandle(conn); err != nil {
				log.Error().Err(err).Caller().Send()
			}
		}()
	}
}

func defaultPushTokenQuery() string {
	return streams.PushTokenQueryParam
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
