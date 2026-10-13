package vision

import (
	"testing"
	"time"
)

func det(x, y, w, h float64, label string, score float64) VisionBBox {
	return VisionBBox{X: x, Y: y, W: w, H: h, Label: label, Score: score}
}

// 同一目标连续两拍：应复用同一 track_id，且坐标连续（不外跳）。
func TestTrackerKeepsIDAcrossFrames(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	out1 := tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)}, base)
	if len(out1) != 1 || out1[0].TrackID == 0 {
		t.Fatalf("第一拍应产生 1 条航迹，got %+v", out1)
	}
	id := out1[0].TrackID

	// 第二拍：目标移动了一点和分数抖动
	out2 := tr.Update([]VisionBBox{det(0.52, 0.5, 0.2, 0.2, "bus", 0.7)}, base.Add(70*time.Millisecond))
	if len(out2) != 1 {
		t.Fatalf("第二拍应仍只有 1 条航迹，got %d", len(out2))
	}
	if out2[0].TrackID != id {
		t.Fatalf("track_id 应保持稳定：%d -> %d", id, out2[0].TrackID)
	}
	if out2[0].X <= out1[0].X || out2[0].X > 0.53 {
		t.Fatalf("坐标应跟上检测：got %v", out2[0].X)
	}
	if out2[0].Miss != 0 {
		t.Fatalf("命中拍 miss 应为 0，got %d", out2[0].Miss)
	}
}

// 检测空帧不应让框消失：hold 内原样保持（不外推），超过 hold 才删除。
func TestTrackerHoldsThroughMissedFrames(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)}, base)
	tr.Update([]VisionBBox{det(0.6, 0.5, 0.2, 0.2, "bus", 0.8)}, base.Add(100*time.Millisecond))

	// 空帧 1：仍在保持，且框停在最后检出位置（不滑）
	out := tr.Update(nil, base.Add(200*time.Millisecond))
	if len(out) != 1 {
		t.Fatalf("空帧应保持 1 条航迹，got %d", len(out))
	}
	if out[0].Miss != 1 {
		t.Fatalf("miss 应为 1，got %d", out[0].Miss)
	}
	if out[0].X != 0.6 {
		t.Fatalf("保持中不应外推（x 应停在 0.6），got %v", out[0].X)
	}

	// 空帧 2：仍在保持，位置/尺寸依旧不变
	out = tr.Update(nil, base.Add(300*time.Millisecond))
	if len(out) != 1 || out[0].Miss != 2 {
		t.Fatalf("第二次空帧应仍保持，got %+v", out)
	}
	if out[0].X != 0.6 || out[0].W != 0.2 || out[0].H != 0.2 {
		t.Fatalf("保持中框应原样不动，got %+v", out[0])
	}

	// 超过 hold（1.2s）后应删除
	out = tr.Update(nil, base.Add(1600*time.Millisecond))
	if len(out) != 0 {
		t.Fatalf("超过 hold 应删除航迹，got %+v", out)
	}
}

// 标签滞回：偶发一次不同标签不应切换，连续 3 拍才切换。
func TestTrackerLabelHysteresis(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)}, base)
	// 一次 bird：不切换
	out := tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bird", 0.5)}, base.Add(100*time.Millisecond))
	if out[0].Label != "bus" {
		t.Fatalf("单次异常标签不应切换，got %q", out[0].Label)
	}
	// 回到 bus：pending 清零
	tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)}, base.Add(200*time.Millisecond))
	// 连续 3 次 bear：切换
	for i := 0; i < 3; i++ {
		out = tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bear", 0.6)}, base.Add(time.Duration(300+i*100)*time.Millisecond))
	}
	if out[0].Label != "bear" {
		t.Fatalf("连续 3 拍同标签应切换为 bear，got %q", out[0].Label)
	}
}

// 两个目标交叉时应保持各自的 id（IoU 匹配的基本健全性）。
func TestTrackerTwoObjectsKeepIdentity(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	out1 := tr.Update([]VisionBBox{
		det(0.2, 0.5, 0.2, 0.2, "person", 0.9),
		det(0.7, 0.5, 0.2, 0.2, "person", 0.8),
	}, base)
	if len(out1) != 2 {
		t.Fatalf("应产生 2 条航迹，got %d", len(out1))
	}
	ids := map[uint64]float64{out1[0].TrackID: out1[0].X, out1[1].TrackID: out1[1].X}
	if len(ids) != 2 {
		t.Fatalf("两个目标应分到不同 id：%+v", ids)
	}

	out2 := tr.Update([]VisionBBox{
		det(0.24, 0.5, 0.2, 0.2, "person", 0.9),
		det(0.66, 0.5, 0.2, 0.2, "person", 0.8),
	}, base.Add(100*time.Millisecond))
	if len(out2) != 2 {
		t.Fatalf("第二拍应仍 2 条，got %d", len(out2))
	}
	// 左目标的 id 应仍对应左边的框
	for _, b := range out2 {
		if x, ok := ids[b.TrackID]; ok {
			if (x < 0.5) != (b.X < 0.5) {
				t.Fatalf("航迹身份串了：id=%d 从 x=%.2f 跳到 x=%.2f", b.TrackID, x, b.X)
			}
		}
	}
}

// 保持中的框必须原地不动：不外推滑行、不收缩，重新检出时才更新。
func TestTrackerHoldStaysPut(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	tr.Update([]VisionBBox{det(0.85, 0.85, 0.1, 0.1, "bus", 0.8)}, base)
	tr.Update([]VisionBBox{det(0.88, 0.88, 0.1, 0.1, "bus", 0.8)}, base.Add(50*time.Millisecond))
	for i := 0; i < 10; i++ {
		out := tr.Update(nil, base.Add(time.Duration(100+i*50)*time.Millisecond))
		for _, b := range out {
			if b.X != 0.88 || b.Y != 0.88 || b.W != 0.1 || b.H != 0.1 {
				t.Fatalf("保持中框应原样停在最后检出位置，got %+v", b)
			}
		}
	}
	// 超过 hold 后删除
	out := tr.Update(nil, base.Add(1600*time.Millisecond))
	if len(out) != 0 {
		t.Fatalf("超过 hold 应删除航迹，got %+v", out)
	}
	// 重新检出：同位置应能接回航迹并更新坐标
	out = tr.Update([]VisionBBox{det(0.9, 0.9, 0.1, 0.1, "bus", 0.8)}, base.Add(1700*time.Millisecond))
	if len(out) != 1 || out[0].X != 0.9 || out[0].Miss != 0 {
		t.Fatalf("重新检出应更新位置，got %+v", out)
	}
}

// 保持中的框尺寸不变：不收缩，也就不会在重新检出时张回原尺寸（避免"框变宽"的呼吸感）。
func TestTrackerHoldKeepsSize(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)}, base)
	tr.Update([]VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)}, base.Add(100*time.Millisecond))
	cur := tr.Update(nil, base.Add(200*time.Millisecond))
	if len(cur) != 1 {
		t.Fatalf("应保持 1 条：%+v", cur)
	}
	for i := 0; i < 5; i++ {
		cur = tr.Update(nil, base.Add(time.Duration(250+i*100)*time.Millisecond))
		if len(cur) != 1 {
			t.Fatalf("保持期间应仍在：%+v", cur)
		}
		if cur[0].W != 0.2 || cur[0].H != 0.2 {
			t.Fatalf("保持中尺寸应不变，got %.3fx%.3f", cur[0].W, cur[0].H)
		}
	}
}

// 尺寸差异大的新检测不应接管旧航迹（否则旧框被「拉长」到新目标）。
func TestTrackerSizeGatePreventsStretch(t *testing.T) {
	tr := NewBoxTracker(1200 * time.Millisecond)
	base := time.Now()

	out1 := tr.Update([]VisionBBox{det(0.5, 0.5, 0.1, 0.1, "bus", 0.8)}, base)
	oldID := out1[0].TrackID
	// 同一位置出现一个 3 倍大的框：应新建航迹，而不是把旧框撑大
	out2 := tr.Update([]VisionBBox{det(0.5, 0.5, 0.3, 0.3, "bus", 0.8)}, base.Add(100*time.Millisecond))
	if len(out2) != 2 {
		t.Fatalf("应新建 1 条航迹（共 2 条）：%+v", out2)
	}
	for _, b := range out2 {
		if b.TrackID == oldID && (b.W > 0.11 || b.H > 0.11) {
			t.Fatalf("旧航迹被撑大了：%+v", b)
		}
	}
}

// trackBoxesForStream：同一流的航迹跨拍保持；Reset 后注册表项被清掉，
// 下次调用创建全新跟踪器（内部无残留航迹）。
func TestStreamTrackerRegistry(t *testing.T) {
	key := "ingest-test-track"
	defer ResetStreamTracker(key)

	out1 := trackBoxesForStream(key, []VisionBBox{det(0.5, 0.5, 0.2, 0.2, "bus", 0.8)})
	if len(out1) != 1 || out1[0].TrackID == 0 {
		t.Fatalf("第一拍应有 1 条带 id 的航迹：%+v", out1)
	}
	out2 := trackBoxesForStream(key, []VisionBBox{det(0.52, 0.5, 0.2, 0.2, "bus", 0.7)})
	if len(out2) != 1 || out2[0].TrackID != out1[0].TrackID {
		t.Fatalf("跨拍 id 应稳定：%+v -> %+v", out1, out2)
	}

	ResetStreamTracker(key)
	out3 := trackBoxesForStream(key, []VisionBBox{det(0.52, 0.5, 0.2, 0.2, "bus", 0.7)})
	if len(out3) != 1 || out3[0].TrackID == 0 || out3[0].Miss != 0 {
		t.Fatalf("Reset 后应能重新开始跟踪：%+v", out3)
	}
}
