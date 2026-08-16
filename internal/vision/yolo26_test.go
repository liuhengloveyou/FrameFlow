package vision

import (
	"math"
	"testing"
)

func TestYoloDet3DLayout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d               []int64
		wantC, wantA    int64
		wantChannelMaj  bool
		wantOK          bool
	}{
		{[]int64{1, 84, 8400}, 84, 8400, true, true},
		{[]int64{1, 8400, 84}, 84, 8400, false, true},
		{[]int64{1, 6, 300}, 0, 0, false, false}, // 端到端 [B,6,N]，交给 parseDetectionLayout
		{[]int64{1, 300, 6}, 0, 0, false, false},
		{[]int64{1, 64, 64}, 0, 0, false, false},
	}
	for _, tc := range cases {
		_, c, a, maj, ok := yoloDet3DLayout(tc.d)
		if ok != tc.wantOK {
			t.Fatalf("shape %v ok=%v want %v", tc.d, ok, tc.wantOK)
		}
		if !ok {
			continue
		}
		if c != tc.wantC || a != tc.wantA || maj != tc.wantChannelMaj {
			t.Fatalf("shape %v got C=%d A=%d maj=%v want C=%d A=%d maj=%v",
				tc.d, c, a, maj, tc.wantC, tc.wantA, tc.wantChannelMaj)
		}
	}
}

func TestPostprocessUltralyticsBCNChannelMajor(t *testing.T) {
	t.Parallel()
	const C, A int64 = 8, 20
	shape := []int64{1, C, A}
	raw := make([]float32, int(C*A))
	// anchor 3: center (160,120), wh (40,30) -> xyxy 140..180, 105..135 in input space
	a := 3
	raw[0*int(A)+a] = 160
	raw[1*int(A)+a] = 120
	raw[2*int(A)+a] = 40
	raw[3*int(A)+a] = 30
	raw[4*int(A)+a] = 10 // class0 logit -> high prob
	raw[5*int(A)+a] = -10
	raw[6*int(A)+a] = -10
	raw[7*int(A)+a] = -10
	for j := 0; j < int(A); j++ {
		if j == a {
			continue
		}
		raw[0*int(A)+j] = 1
		raw[1*int(A)+j] = 1
		raw[2*int(A)+j] = 1
		raw[3*int(A)+j] = 1
		for k := 4; k < int(C); k++ {
			raw[k*int(A)+j] = -20
		}
	}
	labels := []string{"a", "b", "c", "d"}
	boxes, ok := postprocessUltralyticsIfBCN(raw, shape, 640, 480, 0, 0, 1, 0.2, 0.5, false, 10, labels)
	if !ok {
		t.Fatal("expected BCN path")
	}
	if len(boxes) != 1 {
		t.Fatalf("want 1 box got %d", len(boxes))
	}
	b := boxes[0]
	if b.Label != "a" {
		t.Fatalf("label %q", b.Label)
	}
	if math.Abs(b.X-140.0/640.0) > 1e-3 || math.Abs(b.Y-105.0/480.0) > 1e-3 {
		t.Fatalf("xy %v %v", b.X, b.Y)
	}
}

func TestPostprocessUltralyticsBCNAnchorMajor(t *testing.T) {
	t.Parallel()
	const C, A int64 = 8, 20
	shape := []int64{1, A, C}
	raw := make([]float32, int(A*C))
	a := 3
	raw[a*int(C)+0] = 160
	raw[a*int(C)+1] = 120
	raw[a*int(C)+2] = 40
	raw[a*int(C)+3] = 30
	raw[a*int(C)+4] = 10
	raw[a*int(C)+5] = -10
	raw[a*int(C)+6] = -10
	raw[a*int(C)+7] = -10
	for j := 0; j < int(A); j++ {
		if j == a {
			continue
		}
		for k := 0; k < int(C); k++ {
			raw[j*int(C)+k] = 0
		}
		for k := 4; k < int(C); k++ {
			raw[j*int(C)+k] = -20
		}
	}
	labels := []string{"a", "b", "c", "d"}
	boxes, ok := postprocessUltralyticsIfBCN(raw, shape, 640, 480, 0, 0, 1, 0.2, 0.5, false, 10, labels)
	if !ok || len(boxes) != 1 {
		t.Fatalf("ok=%v n=%d", ok, len(boxes))
	}
	if boxes[0].Label != "a" {
		t.Fatal(boxes[0].Label)
	}
}
