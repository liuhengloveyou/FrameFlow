package vision

import "testing"

func TestPointInPolygon(t *testing.T) {
	sq := []NormPoint{{0.2, 0.2}, {0.8, 0.2}, {0.8, 0.8}, {0.2, 0.8}}
	if !pointInPolygon(0.5, 0.5, sq) {
		t.Fatal("center should be inside")
	}
	if pointInPolygon(0.1, 0.1, sq) {
		t.Fatal("outside corner")
	}
}

func TestFilterBoxesByRegions(t *testing.T) {
	boxes := []VisionBBox{
		{X: 0.4, Y: 0.4, W: 0.1, H: 0.1, Score: 0.9, Label: "a"},
		{X: 0.0, Y: 0.0, W: 0.05, H: 0.05, Score: 0.9, Label: "b"},
	}
	regions := [][]NormPoint{{{0.2, 0.2}, {0.8, 0.2}, {0.8, 0.8}, {0.2, 0.8}}}
	out := filterBoxesByRegions(boxes, regions)
	if len(out) != 1 || out[0].Label != "a" {
		t.Fatalf("got %+v", out)
	}
}

func TestParseStreamPolicyGating(t *testing.T) {
	SetStreamAIPolicyResolver(func(streamKey string) (StreamAIPolicy, bool) {
		return StreamAIPolicy{Enabled: false, AlgorithmName: "demo"}, true
	})
	t.Cleanup(func() { SetStreamAIPolicyResolver(nil) })

	boxes, text, err := detectForStream("gb28181://a/b", func(modelPath string, conf float32) ([]VisionBBox, error) {
		t.Fatal("should not run detect")
		return nil, nil
	})
	if err != nil || len(boxes) != 0 || text == "" {
		t.Fatalf("boxes=%v text=%q err=%v", boxes, text, err)
	}
}
