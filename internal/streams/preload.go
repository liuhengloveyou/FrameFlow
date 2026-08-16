package streams

import (
	"fmt"
	"maps"
	"net/url"
	"sync"

	"github.com/liuhengloveyou/FrameFlow/pkg/fferrors"
	"github.com/liuhengloveyou/FrameFlow/pkg/probe"
)

type Preload struct {
	stream *Stream      // Don't include the stream in JSON to avoid leaking secrets.
	Cons   *probe.Probe `json:"consumer"`
	Query  string       `json:"query"`
}

var preloads = map[string]*Preload{}
var preloadsMu sync.Mutex

func AddPreload(name, rawQuery string) error {
	if rawQuery == "" {
		rawQuery = "video&audio"
	}

	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return err
	}

	preloadsMu.Lock()
	defer preloadsMu.Unlock()

	if p := preloads[name]; p != nil {
		p.stream.RemoveConsumer(p.Cons)
	}

	stream := Get(name)
	if stream == nil {
		return fferrors.ErrStreamsStreamNotFoundName(name)
	}
	cons := probe.Create("preload", query)

	if err = stream.AddConsumer(cons); err != nil {
		return err
	}

	preloads[name] = &Preload{stream: stream, Cons: cons, Query: rawQuery}
	return nil
}

func DelPreload(name string) error {
	preloadsMu.Lock()
	defer preloadsMu.Unlock()

	if p := preloads[name]; p != nil {
		p.stream.RemoveConsumer(p.Cons)
		delete(preloads, name)
		return nil
	}

	return fmt.Errorf("streams: preload not found: %s", name)
}

func GetPreloads() map[string]*Preload {
	preloadsMu.Lock()
	defer preloadsMu.Unlock()
	return maps.Clone(preloads)
}
