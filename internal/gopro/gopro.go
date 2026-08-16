package gopro

import (
	"net/http"

	"github.com/liuhengloveyou/FrameFlow/internal/api"
	"github.com/liuhengloveyou/FrameFlow/internal/streams"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/gopro"
)

func Init() {
	streams.HandleFunc("gopro", func(source string) (core.Producer, error) {
		return gopro.Dial(source)
	})

	api.RegisterBizAPI("", "api/gopro", false, "", apiGoPro)
}

func apiGoPro(w http.ResponseWriter, r *http.Request) {
	var items []*api.Source

	for _, host := range gopro.Discovery() {
		items = append(items, &api.Source{Name: host, URL: "gopro://" + host})
	}

	api.ResponseSources(w, items)
}
