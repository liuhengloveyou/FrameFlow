package api

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"
)

func listen(network, address string) {
	ln, err := net.Listen(network, address)
	if err != nil {
		log.Fatal().Err(err).Str("addr", address).
			Msg("[api] listen failed — 端口已被占用时，前端请求会打到旧进程，表现为无本进程日志")
		return
	}

	log.Info().Str("addr", address).Msg("[api] listen")

	server := http.Server{
		Handler:           Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err = server.Serve(ln); err != nil {
		log.Fatal().Err(err).Msg("[api] serve")
	}
}

func tlsListen(network, address, certFile, keyFile string) {
	var cert tls.Certificate
	var err error
	if strings.IndexByte(certFile, '\n') < 0 && strings.IndexByte(keyFile, '\n') < 0 {
		cert, err = tls.LoadX509KeyPair(certFile, keyFile)
	} else {
		cert, err = tls.X509KeyPair([]byte(certFile), []byte(keyFile))
	}
	if err != nil {
		log.Error().Err(err).Caller().Send()
		return
	}

	ln, err := net.Listen(network, address)
	if err != nil {
		log.Error().Err(err).Msg("[api] tls listen")
		return
	}

	log.Info().Str("addr", address).Msg("[api] tls listen")

	server := &http.Server{
		Handler:           Handler,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err = server.ServeTLS(ln, "", ""); err != nil {
		log.Fatal().Err(err).Msg("[api] tls serve")
	}
}
