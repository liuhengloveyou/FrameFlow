package core

import (
	"errors"
	"sync/atomic"
	"time"
)

// Commercial defaults aligned with monibuca Publish.RingSize / live BufferTime.
const (
	AVRingSizeMin     = 20
	AVRingSizeMax     = 1024
	AVRingBufferTime  = 3 * time.Second // keep ≥ this of history before dropping old GOPs
	AVRingMaxBytes    = 32 << 20        // hard memory cap per video track
	AVRingGlowStep    = 5
	AVRingReduceStep  = 5
	AVRingReduceVolN  = 50 // consecutive reduce opportunities before shrink
)

var (
	ErrAVRingClosed  = errors.New("avring: closed")
	ErrAVRingDiscard = errors.New("avring: frame discarded")
	ErrAVRingNoIDR   = errors.New("avring: no idr")
)

// AVRing is the video history ring (one per H264/H265 Receiver).
// Write path mirrors monibuca RingWriter.Step; read path uses per-frame RLock.
type AVRing struct {
	*Ring[AVFrame]
	idrs idrList

	sizeMin int
	sizeMax int
	size    int
	bufMin  time.Duration // BufferRange[0]: min history to retain
	bufCur  time.Duration // BufferRange[1]: duration from oldest IDR to tip

	pool     *Ring[AVFrame]
	poolSize int
	reduceVol int

	bytes atomic.Int64

	LastValue *AVFrame
	status    atomic.Int32 // 0 idle, 1 writing, -1 disposed
	closed    atomic.Bool
}

// NewAVRing creates a production ring (size 20..1024, ~3s GOP history).
func NewAVRing() *AVRing {
	rb := &AVRing{
		sizeMin: AVRingSizeMin,
		sizeMax: AVRingSizeMax,
		size:    AVRingSizeMin,
		bufMin:  AVRingBufferTime,
		Ring:    NewRing[AVFrame](AVRingSizeMin),
	}
	rb.idrs.Init()
	rb.LastValue = &rb.Value
	rb.LastValue.StartWrite()
	return rb
}

func (rb *AVRing) GetIDR() *Ring[AVFrame] {
	if n := rb.idrs.Back(); n != nil {
		return n.Value
	}
	return nil
}

func (rb *AVRing) GetOldestIDR() *Ring[AVFrame] {
	if n := rb.idrs.Front(); n != nil {
		return n.Value
	}
	return nil
}

func (rb *AVRing) Size() int { return rb.size }

func (rb *AVRing) CurrentBufferTime() time.Duration { return rb.bufCur }

func (rb *AVRing) LastSequence() uint32 {
	if rb.LastValue == nil {
		return 0
	}
	return rb.LastValue.Sequence
}

// Write moves pkt into the current write slot and Steps the ring.
// For IDR frames, paramSets are prepended into Packets (Marker=false) so the
// IDR slot is self-decodable — monibuca CheckCodecChange / Mux style.
// Caller must not reuse pkt.Payload after return.
func (rb *AVRing) Write(pkt *Packet, isIDR bool, paramSets [][]byte) {
	if pkt == nil || rb.closed.Load() {
		return
	}
	pkts := make([]Packet, 0, len(paramSets)+1)
	if isIDR {
		for _, ps := range paramSets {
			if len(ps) == 0 {
				continue
			}
			p := Packet{Payload: append([]byte(nil), ps...)}
			p.Timestamp = pkt.Timestamp
			pkts = append(pkts, p)
		}
	}
	cp := *pkt
	pkt.Payload = nil // move
	pkts = append(pkts, cp)
	rb.WriteAU(pkts, isIDR)
}

// WriteAU stores one Access Unit (possibly multiple RTP packets) in a slot.
func (rb *AVRing) WriteAU(pkts []Packet, isIDR bool) {
	if len(pkts) == 0 || rb.closed.Load() {
		return
	}
	f := &rb.Value
	f.IDR = isIDR
	f.Packets = pkts
	f.Packet = Packet{}
	f.ParamSets = nil
	rb.bytes.Add(avFrameBytes(f))
	if isIDR && rb.idrs.Len() == 0 {
		log.Info().
			Uint32("seq", f.Sequence).
			Int("packets", len(f.Packets)).
			Int("payload0", len(f.Packets[0].Payload)).
			Msg("[avring] first IDR AU written to ring")
	}
	rb.Step()
}

// SnapshotFromIDR returns a shallow copy of a media packet from the newest IDR AU.
func (rb *AVRing) SnapshotFromIDR() (*Packet, bool) {
	idr := rb.GetIDR()
	if idr == nil {
		return nil, false
	}
	idr.Value.RLock()
	defer idr.Value.RUnlock()
	if idr.Value.discard || !idr.Value.IDR {
		return nil, false
	}
	pkts := idr.Value.MediaPackets()
	if len(pkts) == 0 {
		return nil, false
	}
	cp := pkts[len(pkts)-1]
	return &cp, true
}

func (rb *AVRing) Close() {
	if !rb.closed.CompareAndSwap(false, true) {
		return
	}
	// Dispose: if idle, unlock write slot so blocked readers wake; if writing, Step end handles it.
	if rb.status.Add(-1) == -1 {
		rb.Value.Unlock()
	}
}

func (rb *AVRing) Closed() bool { return rb.closed.Load() }

func (rb *AVRing) PushIDR() {
	rb.idrs.PushBack(rb.Ring)
}

func (rb *AVRing) durationFrom(from *Ring[AVFrame]) time.Duration {
	if from == nil || rb.LastValue == nil {
		return 0
	}
	// Prefer media timestamp (90kHz RTP clock → duration) when available; else wall clock.
	if rb.LastValue.WriteTime.IsZero() || from.Value.WriteTime.IsZero() {
		return 0
	}
	d := rb.LastValue.WriteTime.Sub(from.Value.WriteTime)
	if d < 0 {
		return 0
	}
	return d
}

func (rb *AVRing) glow(n int) (newItem *Ring[AVFrame]) {
	if n <= 0 {
		return nil
	}
	if newCount := n - rb.poolSize; newCount > 0 {
		newItem = NewRing[AVFrame](newCount).Link(rb.pool)
		rb.poolSize = 0
		rb.pool = nil
	} else {
		newItem = rb.pool.Unlink(n)
		rb.poolSize -= n
		if rb.poolSize == 0 {
			rb.pool = nil
		}
	}
	rb.Link(newItem)
	rb.size += n
	return newItem
}

func avFrameBytes(f *AVFrame) int64 {
	if len(f.Packets) > 0 {
		var n int64
		for i := range f.Packets {
			n += int64(len(f.Packets[i].Payload))
		}
		return n
	}
	n := int64(len(f.Packet.Payload))
	for _, p := range f.ParamSets {
		n += int64(len(p))
	}
	return n
}

func (rb *AVRing) reduce(n int) {
	if n <= 0 || rb.size <= rb.sizeMin {
		return
	}
	if rb.size-n < rb.sizeMin {
		n = rb.size - rb.sizeMin
	}
	r := rb.Unlink(n)
	rb.size -= n
	for i := 0; i < n; i++ {
		if r.Value.StartWrite() {
			rb.bytes.Add(-avFrameBytes(&r.Value))
			rb.poolSize++
			r.Value.Reset()
			r.Value.Unlock()
		} else {
			rb.bytes.Add(-avFrameBytes(&r.Value))
			r.Value.Discard()
			r = r.Prev()
			r.Unlink(1)
		}
		r = r.Next()
	}
	if rb.poolSize > 0 {
		if rb.pool == nil {
			rb.pool = r
		} else {
			rb.pool.Link(r)
		}
	}
}

// Step advances the write pointer after the current slot is filled (monibuca RingWriter.Step).
func (rb *AVRing) Step() (normal bool) {
	isIDR := rb.Value.IDR
	next := rb.Next()
	if isIDR {
		rb.PushIDR()
	}

	if rb.idrs.Len() > 0 {
		oldIDR := rb.idrs.Front()
		rb.bufCur = rb.durationFrom(oldIDR.Value)

		// Memory hard cap: drop oldest GOPs while keeping ≥1 IDR when possible.
		for rb.bytes.Load() > AVRingMaxBytes && rb.idrs.Len() > 1 {
			front := rb.idrs.Front()
			if front == nil {
				break
			}
			rb.idrs.Remove(front)
		}

		if next == rb.idrs.Back().Value {
			if rb.size < rb.sizeMax {
				rb.glow(AVRingGlowStep)
				next = rb.Next()
			}
		} else if next == oldIDR.Value {
			if nextOld := oldIDR.Next(); nextOld != nil && rb.durationFrom(nextOld.Value) > rb.bufMin {
				rb.idrs.Remove(oldIDR)
			} else if rb.size < rb.sizeMax {
				rb.glow(AVRingGlowStep)
				next = rb.Next()
			}
		} else if rb.bufCur > rb.bufMin {
			for tmpP, reduceCount := rb.Next(), 0; reduceCount < AVRingReduceStep; reduceCount++ {
				if tmpP == oldIDR.Value {
					rb.reduceVol = 0
					break
				}
				if tmpP = tmpP.Next(); reduceCount == AVRingReduceStep-1 {
					if rb.size > rb.sizeMin+AVRingReduceStep {
						if rb.reduceVol++; rb.reduceVol > AVRingReduceVolN {
							rb.reduce(AVRingReduceStep)
							next = rb.Next()
							rb.reduceVol = 0
						}
					} else {
						rb.reduceVol = 0
					}
				}
			}
		}
	}

	rb.LastValue = &rb.Value
	nextSeq := rb.LastValue.Sequence + 1

	if rb.status.Add(1) == 1 {
		if normal = next.Value.StartWrite(); normal {
			rb.bytes.Add(-avFrameBytes(&next.Value))
			next.Value.Reset()
			rb.Ring = next
		} else {
			rb.reduce(1)
			rb.glow(1)
			rb.Ring = rb.Next()
			normal = rb.Value.StartWrite()
			if !normal {
				// Should not happen on a fresh node; recover by discarding and retrying once.
				rb.Value.Discard()
				rb.glow(1)
				rb.Ring = rb.Next()
				_ = rb.Value.StartWrite()
			}
		}
		rb.Value.Sequence = nextSeq
		if rb.status.Add(-1) == 0 {
			rb.LastValue.Ready()
		} else {
			// Disposed during Step.
			rb.Value.Unlock()
		}
	}
	return
}
