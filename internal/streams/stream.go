package streams

import (
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
)

type Stream struct {
	producers []*Producer
	consumers []core.Consumer
	mu        sync.Mutex
	pending   atomic.Int32

	// 流附属数据（如 vision JSON）：与 WebRTC DataChannel 等解耦，由订阅者自行消费。
	dataMu   sync.Mutex
	dataSubs map[uint64]func([]byte)
	dataNext atomic.Uint64
}

func NewStream(source any) *Stream {
	switch source := source.(type) {
	case string:
		return &Stream{
			producers: []*Producer{NewProducer(source)},
		}
	case []string:
		s := new(Stream)
		for _, str := range source {
			s.producers = append(s.producers, NewProducer(str))
		}
		return s
	case []any:
		s := new(Stream)
		for _, src := range source {
			str, ok := src.(string)
			if !ok {
				log.Error().Msgf("[stream] NewStream: Expected string, got %v", src)
				continue
			}
			s.producers = append(s.producers, NewProducer(str))
		}
		return s
	case map[string]any:
		return NewStream(source["url"])
	case nil:
		return new(Stream)
	default:
		panic(core.Caller())
	}
}

func (s *Stream) Sources() []string {
	sources := make([]string, 0, len(s.producers))
	for _, prod := range s.producers {
		sources = append(sources, prod.url)
	}
	return sources
}

func (s *Stream) SetSource(source string) {
	for _, prod := range s.producers {
		prod.SetSource(source)
	}
}

func (s *Stream) RemoveConsumer(cons core.Consumer) {
	_ = cons.Stop()

	s.mu.Lock()
	for i, consumer := range s.consumers {
		if consumer == cons {
			s.consumers = append(s.consumers[:i], s.consumers[i+1:]...)
			break
		}
	}
	s.mu.Unlock()

	s.stopProducers()
}

func (s *Stream) AddProducer(prod core.Producer) {
	producer := &Producer{conn: prod, state: stateExternal, url: "external"}
	s.mu.Lock()
	s.producers = append(s.producers, producer)
	s.mu.Unlock()
}

func (s *Stream) RemoveProducer(prod core.Producer) {
	s.mu.Lock()
	for i, producer := range s.producers {
		if producer.conn == prod {
			s.producers = append(s.producers[:i], s.producers[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
}

// HasLiveExternalProducer reports whether an ANNOUNCE/publish producer is attached.
func (s *Stream) HasLiveExternalProducer() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.producers {
		if p != nil && p.state == stateExternal && p.conn != nil {
			return true
		}
	}
	return false
}

func (s *Stream) stopProducers() {
	if s.pending.Load() > 0 {
		log.Trace().Msg("[streams] skip stop pending producer")
		return
	}

	s.mu.Lock()
producers:
	for _, producer := range s.producers {
		for _, track := range producer.receivers {
			if len(track.Senders()) > 0 {
				continue producers
			}
		}
		for _, track := range producer.senders {
			if len(track.Senders()) > 0 {
				continue producers
			}
		}
		producer.stop()
	}
	s.mu.Unlock()
}

func (s *Stream) MarshalJSON() ([]byte, error) {
	var info = struct {
		Producers []*Producer     `json:"producers"`
		Consumers []core.Consumer `json:"consumers"`
	}{
		Producers: s.producers,
		Consumers: s.consumers,
	}
	return json.Marshal(info)
}

// RegisterStreamDataSubscriber 注册本流附属数据订阅者（如将 JSON 写入 DataChannel）。
// streamKey 仅用于 SetStreamDataHooks 回调（与浏览器 /api/webrtc?src= 一致）。
func (s *Stream) RegisterStreamDataSubscriber(streamKey string, send func([]byte)) (cancel func()) {
	if s == nil || send == nil || streamKey == "" {
		return func() {}
	}

	id := s.dataNext.Add(1)

	s.dataMu.Lock()
	if s.dataSubs == nil {
		s.dataSubs = make(map[uint64]func([]byte))
	}
	first := len(s.dataSubs) == 0
	s.dataSubs[id] = send
	fnFirst := onFirstStreamDataSubscriber
	s.dataMu.Unlock()

	if first && fnFirst != nil {
		fnFirst(streamKey)
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			var fnLast func(string)
			s.dataMu.Lock()
			if s.dataSubs != nil {
				delete(s.dataSubs, id)
				if len(s.dataSubs) == 0 {
					s.dataSubs = nil
					fnLast = onLastStreamDataUnsubscriber
				}
			}
			s.dataMu.Unlock()
			if fnLast != nil {
				fnLast(streamKey)
			}
		})
	}
}

// BroadcastStreamData 向本流所有附属数据订阅者推送 payload（通常为 JSON UTF-8）。
func (s *Stream) BroadcastStreamData(payload []byte) {
	if s == nil || len(payload) == 0 {
		return
	}

	s.dataMu.Lock()
	if len(s.dataSubs) == 0 {
		s.dataMu.Unlock()
		return
	}
	senders := make([]func([]byte), 0, len(s.dataSubs))
	for _, fn := range s.dataSubs {
		senders = append(senders, fn)
	}
	s.dataMu.Unlock()

	for _, fn := range senders {
		if fn != nil {
			fn(payload)
		}
	}
}
