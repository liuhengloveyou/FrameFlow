package core

import (
	"context"
	"time"
)

// RingReader locks successive AVFrame slots (monibuca ring-reader).
type RingReader struct {
	*Ring[AVFrame]
	locked bool
	Count  int
	ring   *AVRing
}

func (r *RingReader) StartRead(node *Ring[AVFrame]) error {
	r.Ring = node
	if r.Value.discard {
		return ErrAVRingDiscard
	}
	r.Value.RLock()
	r.locked = true
	r.Count++
	return nil
}

func (r *RingReader) StopRead() {
	if r.locked {
		r.Value.RUnlock()
		r.locked = false
	}
}

func (r *RingReader) ReadNext() error {
	return r.Read(r.Next())
}

func (r *RingReader) Read(node *Ring[AVFrame]) error {
	r.StopRead()
	return r.StartRead(node)
}

const (
	readStateInit = iota
	readStateFirst
	readStateNormal
	readStateWaitKey
)

// AVRingReader is the live reader: start at newest IDR, follow tip, jump on lag/discard.
type AVRingReader struct {
	RingReader
	State    byte
	FirstSeq uint32
	ring     *AVRing
	pktIdx   int // index into current AU Packets
}

func NewAVRingReader(ring *AVRing) *AVRingReader {
	return &AVRingReader{ring: ring, RingReader: RingReader{ring: ring}}
}

func (r *AVRingReader) auLen() int {
	if r.Ring == nil {
		return 0
	}
	return len(r.Value.MediaPackets())
}

func (r *AVRingReader) takePacket() (*Packet, bool) {
	pkts := r.Value.MediaPackets()
	if r.pktIdx >= len(pkts) {
		return nil, false
	}
	cp := pkts[r.pktIdx]
	r.pktIdx++
	return &cp, true
}

// ReadFrame advances one RTP packet under REAL subscribe semantics.
// Multi-packet AUs (monibuca) are drained packet-by-packet before moving slots.
func (r *AVRingReader) ReadFrame(ctx context.Context) (*Packet, error) {
	for {
		if r.ring == nil || r.ring.Closed() {
			return nil, ErrAVRingClosed
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Still draining current AU.
		if r.locked && r.pktIdx > 0 && r.pktIdx < r.auLen() {
			if p, ok := r.takePacket(); ok {
				return p, nil
			}
		}

		switch r.State {
		case readStateInit:
			start := r.ring.Ring
			idr := r.ring.GetIDR()
			if idr != nil {
				start = idr
				r.State = readStateFirst
			} else {
				r.State = readStateWaitKey
			}
			if start == nil {
				if err := r.waitLockStart(ctx); err != nil {
					return nil, err
				}
			} else if err := r.lockNode(ctx, start); err != nil {
				return nil, err
			}
			r.pktIdx = 0
			r.FirstSeq = r.Value.Sequence
			if r.State == readStateWaitKey && !r.Value.IDR {
				continue
			}

		case readStateFirst:
			if idr := r.ring.GetIDR(); idr != nil && idr.Value.Sequence != r.FirstSeq {
				if err := r.lockNode(ctx, idr); err != nil {
					return nil, err
				}
				r.pktIdx = 0
			} else if err := r.advance(ctx); err != nil {
				return nil, err
			}
			r.State = readStateNormal

		case readStateWaitKey:
			if r.Ring == nil {
				if err := r.waitLockStart(ctx); err != nil {
					return nil, err
				}
				r.pktIdx = 0
			} else if err := r.advance(ctx); err != nil {
				return nil, err
			}
			if !r.Value.IDR {
				continue
			}
			r.State = readStateNormal
			r.FirstSeq = r.Value.Sequence

		case readStateNormal:
			if err := r.advance(ctx); err != nil {
				return nil, err
			}
		}

		if p, ok := r.takePacket(); ok {
			return p, nil
		}
		// Empty AU — keep going.
	}
}

func (r *AVRingReader) advance(ctx context.Context) error {
	if r.Ring == nil {
		r.pktIdx = 0
		return r.waitLockStart(ctx)
	}
	if err := r.lockNode(ctx, r.Next()); err != nil {
		return err
	}
	r.pktIdx = 0
	// Slow reader recovery (monibuca: lag > Size/2 → latest IDR).
	last := r.ring.LastSequence()
	if last > r.Value.Sequence && last-r.Value.Sequence > uint32(r.ring.Size()/2) {
		if idr := r.ring.GetIDR(); idr != nil && idr.Value.Sequence > r.Value.Sequence {
			if err := r.lockNode(ctx, idr); err != nil {
				return err
			}
			r.pktIdx = 0
		}
	}
	return nil
}

// waitLockStart parks until an IDR or write tip exists, then locks it.
func (r *AVRingReader) waitLockStart(ctx context.Context) error {
	for {
		if r.ring == nil || r.ring.Closed() {
			return ErrAVRingClosed
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if idr := r.ring.GetIDR(); idr != nil {
			return r.lockNode(ctx, idr)
		}
		if tip := r.ring.Ring; tip != nil {
			return r.lockNode(ctx, tip)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Millisecond):
		}
	}
}

func (r *AVRingReader) lockNode(ctx context.Context, node *Ring[AVFrame]) error {
	r.StopRead()
	for {
		if r.ring == nil || r.ring.Closed() {
			return ErrAVRingClosed
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if node == nil {
			return ErrAVRingDiscard
		}
		if node.Value.discard {
			idr := r.ring.GetIDR()
			if idr == nil {
				return ErrAVRingDiscard
			}
			node = idr
			continue
		}
		if node.Value.TryRLock() {
			if node.Value.discard {
				node.Value.RUnlock()
				idr := r.ring.GetIDR()
				if idr == nil {
					return ErrAVRingDiscard
				}
				node = idr
				continue
			}
			r.Ring = node
			r.locked = true
			r.Count++
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Microsecond):
		}
	}
}

// SeekLatestIDR repositions to the newest IDR AU (Sender drop recovery).
// pktIdx is reset so SPS/PPS prefix is emitted before the IDR NAL.
func (r *AVRingReader) SeekLatestIDR(ctx context.Context) error {
	if r.ring == nil {
		return ErrAVRingClosed
	}
	idr := r.ring.GetIDR()
	if idr == nil {
		return ErrAVRingNoIDR
	}
	r.State = readStateNormal
	r.pktIdx = 0
	return r.lockNode(ctx, idr)
}

func (r *AVRingReader) Close() {
	r.StopRead()
}
