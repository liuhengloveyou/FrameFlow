package core

import "testing"

func TestDTSEstimatorBFrames(t *testing.T) {
	var est DTSEstimator
	// 模拟解码序 I(0), P(120), B(40), B(80) —— RTP Timestamp=PTS
	ptsSeq := []uint32{0, 120, 40, 80}
	var prevDTS uint32
	for i, pts := range ptsSeq {
		dts := est.Feed(pts)
		cts := int32(pts - dts)
		if i > 0 && dts < prevDTS {
			t.Fatalf("DTS must be non-decreasing: prev=%d dts=%d", prevDTS, dts)
		}
		_ = cts
		prevDTS = dts
	}
	// 含 B 帧后，P 帧应带正 CTS，B 帧 CTS 可为负或较小
	est2 := DTSEstimator{}
	_ = est2.Feed(0)
	dtsP := est2.Feed(120)
	if dtsP != 120 {
		t.Fatalf("before B detected, dts should equal pts, got %d", dtsP)
	}
	dtsB := est2.Feed(40)
	ctsB := int32(uint32(40) - dtsB)
	if dtsB < 120 {
		// 被抬到 prevDTS
		t.Fatalf("dts should clamp to prev, got %d", dtsB)
	}
	if ctsB >= 0 {
		// 40 - 120 = -80
		t.Fatalf("expected negative CTS for early B, got %d (dts=%d)", ctsB, dtsB)
	}
}
