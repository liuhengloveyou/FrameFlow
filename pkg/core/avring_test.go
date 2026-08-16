package core

import (
	"context"
	"encoding/binary"
	"testing"
	"time"
)

func TestAVRingWriteIDRParamSets(t *testing.T) {
	r := NewAVRing()
	defer r.Close()

	sps := []byte{0x67, 0x42, 0x00, 0x1e}
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	r.Write(pkt(1, avccNAL(5, []byte{0xAA})), true, [][]byte{sps, pps})

	idr := r.GetIDR()
	if idr == nil {
		t.Fatal("expected IDR")
	}
	idr.Value.RLock()
	defer idr.Value.RUnlock()
	pkts := idr.Value.MediaPackets()
	if len(pkts) != 3 {
		t.Fatalf("want SPS+PPS+IDR (3 packets), got %d", len(pkts))
	}
	if string(pkts[0].Payload) != string(sps) || string(pkts[1].Payload) != string(pps) {
		t.Fatalf("param sets mismatch: %v", pkts)
	}
	if pkts[0].Marker || pkts[1].Marker {
		t.Fatal("SPS/PPS must not set Marker (avoid early RTSP flush)")
	}
}

func TestAVRingWriteMoveAndReadFromIDR(t *testing.T) {
	r := NewAVRing()
	defer r.Close()

	p1 := pkt(1, avccNAL(1, []byte{0x01}))
	r.Write(p1, false, nil)
	if p1.Payload != nil {
		t.Fatal("expected payload moved (nil after Write)")
	}

	r.Write(pkt(2, avccNAL(5, []byte{0xAA, 0xBB})), true, nil)
	r.Write(pkt(3, avccNAL(1, []byte{0x01})), false, nil)

	rr := NewAVRingReader(r)
	defer rr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := rr.ReadFrame(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != 2 {
		t.Fatalf("want IDR ts=2, got %d", got.Timestamp)
	}
	got, err = rr.ReadFrame(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != 3 {
		t.Fatalf("want P ts=3, got %d", got.Timestamp)
	}
}

func TestAVRingSnapshotFromIDR(t *testing.T) {
	r := NewAVRing()
	defer r.Close()

	r.Write(pkt(10, avccNAL(5, []byte{1})), true, nil)
	r.Write(pkt(11, avccNAL(1, []byte{2})), false, nil)

	snap, ok := r.SnapshotFromIDR()
	if !ok || snap.Timestamp != 10 {
		t.Fatalf("snapshot=%v ok=%v", snap, ok)
	}
}

func TestAVRingWaitForIDR(t *testing.T) {
	r := NewAVRing()
	defer r.Close()

	rr := NewAVRingReader(r)
	defer rr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan *Packet, 1)
	go func() {
		p, err := rr.ReadFrame(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		done <- p
	}()

	time.Sleep(20 * time.Millisecond)
	r.Write(pkt(1, avccNAL(1, []byte{1})), false, nil)
	r.Write(pkt(2, avccNAL(5, []byte{9})), true, nil)

	select {
	case p := <-done:
		if p.Timestamp != 2 {
			t.Fatalf("want IDR ts=2, got %d", p.Timestamp)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for IDR")
	}
}

func TestAVRingSeekLatestIDR(t *testing.T) {
	r := NewAVRing()
	defer r.Close()

	r.Write(pkt(1, avccNAL(5, []byte{1})), true, nil)
	r.Write(pkt(2, avccNAL(1, []byte{2})), false, nil)
	r.Write(pkt(3, avccNAL(5, []byte{3})), true, nil)

	rr := NewAVRingReader(r)
	defer rr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := rr.ReadFrame(ctx); err != nil {
		t.Fatal(err)
	}
	if err := rr.SeekLatestIDR(ctx); err != nil {
		t.Fatal(err)
	}
	pkts := rr.Value.MediaPackets()
	if len(pkts) == 0 || pkts[len(pkts)-1].Timestamp != 3 {
		t.Fatalf("seek should hold latest IDR ts=3, got %v", pkts)
	}
}

func TestPacketIsIDR_AVCC(t *testing.T) {
	if !packetIsIDR(CodecH264, false, avccNAL(5, []byte{0xFF})) {
		t.Fatal("expected H264 AVCC IDR")
	}
	if packetIsIDR(CodecH264, false, avccNAL(1, []byte{0x01})) {
		t.Fatal("expected non-IDR")
	}
}

func TestPacketIsIDR_H264RTP(t *testing.T) {
	if !packetIsIDR(CodecH264, true, []byte{5, 0x00}) {
		t.Fatal("single NAL IDR")
	}
	if !packetIsIDR(CodecH264, true, []byte{28, 0x80 | 5, 0x00}) {
		t.Fatal("FU-A IDR start")
	}
	if packetIsIDR(CodecH264, true, []byte{1, 0x00}) {
		t.Fatal("P should not be IDR")
	}
}

func pkt(ts uint32, payload []byte) *Packet {
	p := &Packet{Payload: payload}
	p.Timestamp = ts
	return p
}

func avccNAL(nalType byte, body []byte) []byte {
	n := 1 + len(body)
	b := make([]byte, 4+n)
	binary.BigEndian.PutUint32(b, uint32(n))
	b[4] = nalType
	copy(b[5:], body)
	return b
}
