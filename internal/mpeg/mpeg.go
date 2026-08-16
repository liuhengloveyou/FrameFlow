package mpeg

import (
	"net/http"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/aac"
	"github.com/liuhengloveyou/FrameFlow/pkg/mpegts"
)

func Init() {
	api.RegisterBizAPI("", "api/stream.ts", false, "", apiHandle)
	api.RegisterBizAPI("", "api/stream.aac", false, "", apiStreamAAC)
}

func apiHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		outputMpegTS(w, r)
	} else {
		inputMpegTS(w, r)
	}
}

func outputMpegTS(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")
	stream := streams.Get(src)
	if stream == nil {
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	cons := mpegts.NewConsumer()
	cons.WithRequest(r)

	if err := stream.AddConsumer(cons); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "video/mp2t")

	_, _ = cons.WriteTo(w)

	stream.RemoveConsumer(cons)
}

func inputMpegTS(w http.ResponseWriter, r *http.Request) {
	dst := r.URL.Query().Get("dst")
	stream := streams.Get(dst)
	if stream == nil {
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	client, err := mpegts.Open(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stream.AddProducer(client)
	defer stream.RemoveProducer(client)

	if err = client.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func apiStreamAAC(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("src")
	stream := streams.Get(src)
	if stream == nil {
		http.Error(w, api.StreamNotFound, http.StatusNotFound)
		return
	}

	cons := aac.NewConsumer()
	cons.WithRequest(r)

	if err := stream.AddConsumer(cons); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "audio/aac")

	_, _ = cons.WriteTo(w)

	stream.RemoveConsumer(cons)
}
