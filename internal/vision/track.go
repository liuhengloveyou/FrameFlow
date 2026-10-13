package vision

import (
	"math"
	"os"
	"strconv"
	"sync"
	"time"
)

// 航迹跟踪（SORT 风格简化版）：把逐帧检测框变成有身份的连续航迹。
//
// 要解决的问题：检测每帧坐标有抖动、置信度在阈值附近波动时会偶发空帧，
// 前端若整份替换框数组，框就会反复「重建 / 消失 / 跳位」，观感就是闪烁。
//
// 做法：按流维护一个 BoxTracker，每拍用贪心 IoU 把新检测匹配到已有航迹：
//   - 命中：更新位置/分数，标签带滞回（同一新标签连续 3 拍才切换，避免乱跳）；
//   - 未命中：航迹在 hold 时长内**原样保持**（框停在最后一次真实检出位置，
//     不外推、不收缩），只兜住检测的瞬时抖动；
//   - 超过 hold 未再命中：删除航迹。
//
// 明确不做匀速外推：目标驶走/丢失后外推会让框继续滑行（观感「一直滑」），
// 保持中收缩则会在重新检出时张回原尺寸（观感「框变宽了」）。保持中的框由
// 前端以半透明渲染（miss>0），已经足够表达「这不是真实检测」。
//
// 输出框带 TrackID / Miss（连续丢失拍数），前端据此做插值平滑与「预测中」样式。
// 注意：只作用于 DataChannel 的预览 payload；告警（NotifyDetection）仍用原始检测，
// 不会因为航迹保持而产生陈旧的告警。

const (
	defaultTrackHoldMS = 260 // 目标丢失后的保持时长（15fps 下约覆盖 4 拍抖动）
)

// boxTrack 单条航迹。
type boxTrack struct {
	id           uint64
	x, y, w, h   float64 // 归一化 0~1（当前帧位置，丢失时停在最后检出位置）
	label        string
	score        float64
	text         string
	pendingLabel string
	pendingCount int
	lastSeen     time.Time
	misses       int
}

// BoxTracker 单路流的航迹跟踪器。worker 对单流是单 goroutine 串行调用，
// 仍加锁以便将来复用（如被多个入口调用）。
type BoxTracker struct {
	mu     sync.Mutex
	tracks map[uint64]*boxTrack
	nextID uint64
	hold   time.Duration
}

// NewBoxTracker 创建跟踪器。hold<=0 用默认值。
func NewBoxTracker(hold time.Duration) *BoxTracker {
	if hold <= 0 {
		hold = trackHoldFromEnv()
	}
	return &BoxTracker{tracks: map[uint64]*boxTrack{}, hold: hold}
}

func trackHoldFromEnv() time.Duration {
	if v := os.Getenv("FRAMEFLOW_VISION_TRACK_HOLD_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return defaultTrackHoldMS * time.Millisecond
}

// TrackEnabled 是否启用航迹跟踪（FRAMEFLOW_VISION_TRACK=0 关闭，透传原始框）。
func TrackEnabled() bool {
	if v := os.Getenv("FRAMEFLOW_VISION_TRACK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n != 0
		}
	}
	return true
}

// 按流的跟踪器注册表。worker 对单流串行调用，sync.Map 仅为跨流并发安全。
var streamTrackers sync.Map // streamKey -> *BoxTracker

// trackBoxesForStream 对该流跑一拍跟踪；未启用时原样返回。
func trackBoxesForStream(streamKey string, dets []VisionBBox) []VisionBBox {
	if !TrackEnabled() {
		return dets
	}
	v, ok := streamTrackers.Load(streamKey)
	var tr *BoxTracker
	if ok {
		tr = v.(*BoxTracker)
	} else {
		tr = NewBoxTracker(0)
		streamTrackers.Store(streamKey, tr)
	}
	return tr.Update(dets, time.Now())
}

// ResetStreamTracker 清空该流航迹（worker 退出 / 解码重同步时调用）。
func ResetStreamTracker(streamKey string) {
	if v, ok := streamTrackers.LoadAndDelete(streamKey); ok {
		v.(*BoxTracker).Reset()
	}
}

// Update 送入一拍检测结果，返回当前应展示的航迹框（含丢失保持中的框）。
// at 为本拍时刻（用于 hold 超时判定）；调用方每拍调用一次，空检测也要调用
// （让航迹超时删除）。
func (t *BoxTracker) Update(dets []VisionBBox, at time.Time) []VisionBBox {
	t.mu.Lock()
	defer t.mu.Unlock()

	alive := make([]*boxTrack, 0, len(t.tracks))
	for _, tr := range t.tracks {
		if at.Sub(tr.lastSeen) > t.hold {
			delete(t.tracks, tr.id)
			continue
		}
		alive = append(alive, tr)
	}
	if len(alive) == 0 && len(dets) == 0 {
		return nil
	}

	// 贪心匹配：检测按分数降序，优先吃掉 IoU 最大的航迹。
	type pair struct {
		iou   float64
		cost  float64
		det   int
		track *boxTrack
	}
	var pairs []pair
	for di := range dets {
		d := &dets[di]
		for _, tr := range alive {
			iou := trackIoU(d.X, d.Y, d.W, d.H, tr.x, tr.y, tr.w, tr.h)
			if iou < 0.1 {
				continue
			}
			// 尺寸门限：面积差 2.5 倍以上视为不同目标，不匹配。
			// 否则一个遗留航迹会被新目标「撑大/拉长」后接管（观感就是框变长）。
			da, ta := d.W*d.H, tr.w*tr.h
			if da > 0 && ta > 0 {
				small, big := da, ta
				if ta < da {
					small, big = ta, da
				}
				if small/big < 0.4 {
					continue
				}
			}
			cost := 1 - iou
			if d.Label != tr.label {
				cost += 0.15 // 同类优先，减少标签被抢
			}
			pairs = append(pairs, pair{iou: iou, cost: cost, det: di, track: tr})
		}
	}
	// 简单选择排序（框数量级 < 20，无需堆）
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].cost < pairs[i].cost {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	takenDet := make([]bool, len(dets))
	takenTrk := make(map[uint64]bool)
	for _, p := range pairs {
		if takenDet[p.det] || takenTrk[p.track.id] {
			continue
		}
		takenDet[p.det] = true
		takenTrk[p.track.id] = true
		t.matchLocked(p.track, &dets[p.det], at)
	}

	// 未匹配检测 → 新航迹
	for di := range dets {
		if takenDet[di] {
			continue
		}
		t.nextID++
		d := &dets[di]
		t.tracks[t.nextID] = &boxTrack{
			id:       t.nextID,
			x:        d.X, y: d.Y, w: d.W, h: d.H,
			label:    d.Label,
			score:    d.Score,
			text:     d.Text,
			lastSeen: at,
		}
	}

	// 未匹配航迹 → 原样保持（不外推、不收缩），只累计丢失拍数。
	for _, tr := range alive {
		if takenTrk[tr.id] {
			continue
		}
		tr.misses++
	}

	out := make([]VisionBBox, 0, len(t.tracks))
	for _, tr := range t.tracks {
		out = append(out, VisionBBox{
			X: tr.x, Y: tr.y, W: tr.w, H: tr.h,
			Label:   tr.label,
			Score:   tr.score,
			Text:    tr.text,
			TrackID: tr.id,
			Miss:    tr.misses,
		})
	}
	return out
}

// matchLocked 用一帧检测更新航迹（调用方持有锁）。
func (t *BoxTracker) matchLocked(tr *boxTrack, d *VisionBBox, at time.Time) {
	tr.x, tr.y, tr.w, tr.h = d.X, d.Y, d.W, d.H
	tr.score = 0.5*tr.score + 0.5*d.Score
	if d.Text != "" {
		tr.text = d.Text
	}
	// 标签滞回：不同标签连续 3 拍才切换，避免 bird/bear 之类逐帧横跳
	if d.Label != tr.label {
		if tr.pendingLabel == d.Label {
			tr.pendingCount++
		} else {
			tr.pendingLabel, tr.pendingCount = d.Label, 1
		}
		if tr.pendingCount >= 3 {
			tr.label = d.Label
			tr.pendingLabel, tr.pendingCount = "", 0
		}
	} else {
		tr.pendingLabel, tr.pendingCount = "", 0
	}
	tr.lastSeen = at
	tr.misses = 0
}

// Reset 清空航迹（流重启/重同步时用）。
func (t *BoxTracker) Reset() {
	t.mu.Lock()
	t.tracks = map[uint64]*boxTrack{}
	t.mu.Unlock()
}

func trackIoU(ax, ay, aw, ah, bx, by, bw, bh float64) float64 {
	if aw <= 0 || ah <= 0 || bw <= 0 || bh <= 0 {
		return 0
	}
	x1 := math.Max(ax, bx)
	y1 := math.Max(ay, by)
	x2 := math.Min(ax+aw, bx+bw)
	y2 := math.Min(ay+ah, by+bh)
	if x2 <= x1 || y2 <= y1 {
		return 0
	}
	inter := (x2 - x1) * (y2 - y1)
	union := aw*ah + bw*bh - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}
