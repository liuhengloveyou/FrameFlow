package core

import (
	"sync"
	"sync/atomic"
	"time"
)

// AVFrame is one ring slot: writer exclusive lock, readers shared lock.
// Semantics follow monibuca DataFrame (StartWrite / Ready / discard).
//
// For video, Packets holds one Access Unit (same RTP timestamp), matching
// monibuca VideoFrame.Packets. IDR AUs prepend SPS/PPS so SeekLatestIDR is
// self-decodable.
type AVFrame struct {
	sync.RWMutex
	discard   bool
	Sequence  uint32
	IDR       bool
	WriteTime time.Time
	// Packets: RTP packets for this AU (owned Payloads). Preferred over Packet.
	Packets []Packet
	// Packet: legacy single-packet slot (tests / non-aggregated RAW).
	Packet Packet
	// ParamSets: deprecated side-channel; prefer Packets prefix. Kept for
	// readers that still check it as a fallback.
	ParamSets [][]byte
}

func (f *AVFrame) Reset() {
	f.IDR = false
	f.Packets = nil
	f.Packet = Packet{}
	f.ParamSets = nil
}

func (f *AVFrame) Discard() {
	f.discard = true
	f.Reset()
}

// StartWrite tries to take the write lock for reuse.
// If a reader still holds the slot, marks discard and returns false (caller must glow).
func (f *AVFrame) StartWrite() bool {
	if f.discard {
		return false
	}
	if f.TryLock() {
		return true
	}
	f.discard = true
	return false
}

func (f *AVFrame) Ready() {
	f.WriteTime = time.Now()
	f.Unlock()
}

// MediaPackets returns AU packets (Packets, or legacy Packet, with ParamSets prefix).
func (f *AVFrame) MediaPackets() []Packet {
	if len(f.Packets) > 0 {
		return f.Packets
	}
	n := 0
	if len(f.ParamSets) > 0 && f.IDR {
		n += len(f.ParamSets)
	}
	if len(f.Packet.Payload) > 0 || f.Packet.Marker {
		n++
	}
	if n == 0 {
		return nil
	}
	out := make([]Packet, 0, n)
	if f.IDR {
		ts := f.Packet.Timestamp
		for _, ps := range f.ParamSets {
			if len(ps) == 0 {
				continue
			}
			p := Packet{Payload: ps}
			p.Timestamp = ts
			// Marker=false: do not flush PS alone (monibuca / RTSP interleaved).
			out = append(out, p)
		}
	}
	if len(f.Packet.Payload) > 0 || f.Packet.Marker {
		out = append(out, f.Packet)
	}
	return out
}

// --- IDR linked list (monibuca IDRList) ---

type idrNode struct {
	Value *Ring[AVFrame]
	next  atomic.Pointer[idrNode]
	prev  atomic.Pointer[idrNode]
}

func (n *idrNode) Next() *idrNode { return n.next.Load() }
func (n *idrNode) Prev() *idrNode { return n.prev.Load() }

type idrList struct {
	head atomic.Pointer[idrNode]
	tail atomic.Pointer[idrNode]
	len  atomic.Int32
}

func (l *idrList) Init() {
	l.head.Store(nil)
	l.tail.Store(nil)
	l.len.Store(0)
}

func (l *idrList) Len() int { return int(l.len.Load()) }

func (l *idrList) Front() *idrNode { return l.head.Load() }
func (l *idrList) Back() *idrNode  { return l.tail.Load() }

func (l *idrList) PushBack(v *Ring[AVFrame]) {
	node := &idrNode{Value: v}
	l.len.Add(1)
	tail := l.tail.Load()
	node.prev.Store(tail)
	if tail != nil {
		tail.next.Store(node)
	}
	l.tail.Store(node)
	if l.head.Load() == nil {
		l.head.Store(node)
	}
}

// Remove drops the oldest IDR node only when it is still the front (GOP eviction).
func (l *idrList) Remove(node *idrNode) {
	head := l.head.Load()
	if head != node {
		return
	}
	next := head.next.Load()
	l.head.Store(next)
	if next != nil {
		next.prev.Store(nil)
	} else {
		l.tail.Store(nil)
	}
	l.len.Add(-1)
}
