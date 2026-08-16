package core

// DTSEstimator 根据乱序的 PTS（如含 B 帧的 RTP）估计单调的 DTS。
// 对齐 monibuca pkg/util.DTSEstimator：PTS 回退时判定存在 B 帧，
// 用滑动窗口中最小 PTS 作为 DTS，并保证 DTS 不回退。
type DTSEstimator struct {
	hasB     bool
	prevPTS  uint32
	prevDTS  uint32
	cache    []uint32
	interval uint32
}

func (d *DTSEstimator) add(pts uint32) {
	i := 0
	l := len(d.cache)
	if l >= 4 {
		l--
		d.cache = append(d.cache[:0], d.cache[1:]...)[:l]
	}
	for ; i < l; i++ {
		if d.cache[i] > pts {
			break
		}
	}
	d.cache = append(d.cache, pts)
	d.cache = append(d.cache[:i+1], d.cache[i:l]...)
	d.cache[i] = pts
}

// Feed 输入当前帧 PTS，返回估计的 DTS。CTS = PTS - DTS（有符号，按 uint32 模运算）。
func (d *DTSEstimator) Feed(pts uint32) uint32 {
	var interval uint32
	if pts > d.prevPTS {
		interval = pts - d.prevPTS
	} else {
		interval = d.prevPTS - pts
	}
	if d.interval > 0 && interval > 10*d.interval {
		*d = DTSEstimator{}
	}
	d.interval = interval
	d.add(pts)
	dts := pts
	if !d.hasB {
		if pts < d.prevPTS {
			d.hasB = true
			dts = d.cache[0]
		}
	} else {
		dts = d.cache[0]
	}
	if d.prevDTS > dts {
		dts = d.prevDTS
	}
	d.prevPTS = pts
	d.prevDTS = dts
	return dts
}
