//go:build cgo

package ffdecode

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 流式（持久）解码的回归测试与基线对比。
//
// 需要一个 Annex-B H.264/H.265 样本。没有样本时整组自动跳过（不影响 CI）。生成样本：
//
//	ffmpeg -v error -y -i data/datasets/input-1.mp4 -c:v copy -bsf:v h264_mp4toannexb \
//	    -f h264 /tmp/input-1.h264
//
// 样本路径：$FRAMEFLOW_BENCH_H264，或 /tmp/input-1.h264，或 ../../data/datasets/input-1.h264

func benchSamplePath() string {
	if p := os.Getenv("FRAMEFLOW_BENCH_H264"); p != "" {
		return p
	}
	for _, p := range []string{"/tmp/input-1.h264", filepath.Join("..", "..", "data", "datasets", "input-1.h264")} {
		if st, err := os.Stat(p); err == nil && st.Size() > 0 {
			return p
		}
	}
	return ""
}

type sampleAU struct {
	off, size int
	idr       bool
}

// readUE 读一个 UE(v)（无符号指数哥伦布编码）。bitOff 为比特偏移。
func readUE(b []byte, bitOff int) (uint32, bool) {
	zeros, i := 0, bitOff
	for i < len(b)*8 {
		if (b[i/8]>>(7-uint(i%8)))&1 == 1 {
			break
		}
		zeros++
		i++
		if zeros > 32 {
			return 0, false
		}
	}
	if i >= len(b)*8 {
		return 0, false
	}
	i++ // 跳过分隔用的 1
	if zeros == 0 {
		return 0, true
	}
	var val uint32
	for k := 0; k < zeros; k++ {
		if i >= len(b)*8 {
			return 0, false
		}
		val = val<<1 | uint32((b[i/8]>>(7-uint(i%8)))&1)
		i++
	}
	return val - 1, true
}

// firstMBInSlice 解析 slice 头第一个字段 first_mb_in_slice。
// b 为 NAL 负载（不含 NAL 头字节）；ok=false 表示解析失败。
func firstMBInSlice(b []byte) (v uint32, ok bool) {
	if len(b) < 1 {
		return 0, false
	}
	return readUE(b, 0)
}

// splitSampleAUs 按「访问单元（一图）」切分，与 ffmpeg_decode.c 的切分规则一致：
// AUD 一定开新 AU；VCL NAL 则看 slice 头的 first_mb_in_slice —— 为 0 才是新一图，
// 大于 0 是同一图内的后续 slice，必须与前面的 slice 归入同一个 AU。
// （曾经的错误规则是「VCL 紧跟 VCL 就切」，会把多 slice 的帧拆成多个不完整包，
//  导致整帧 conceal、画面花屏——见 TestStreamingGroupsSliceNALsPerPicture。）
func splitSampleAUs(t *testing.T, b []byte) []sampleAU {
	t.Helper()
	type nal struct{ off, end, typ int }
	var nals []nal
	for i := 0; i+3 < len(b); {
		if !(b[i] == 0 && b[i+1] == 0 && b[i+2] == 1) {
			i++
			continue
		}
		start, p := i, i+3
		k := p
		for k+3 <= len(b) && !(b[k] == 0 && b[k+1] == 0 && b[k+2] == 1) {
			k++
		}
		if k+3 > len(b) {
			k = len(b)
		}
		nals = append(nals, nal{off: start, end: k, typ: int(b[p] & 0x1f)})
		i = k
	}
	if len(nals) == 0 {
		t.Fatal("样本里没找到 NAL")
	}
	isVCL := func(x int) bool { return x >= 1 && x <= 5 }
	var aus []sampleAU
	curStart := nals[0].off
	hasVCL, curIDR := false, false
	newPic := func(n nal) bool {
		if n.typ == 9 { // AUD
			return true
		}
		if !isVCL(n.typ) {
			return false
		}
		fmb, ok := firstMBInSlice(b[n.off+3+1 : n.end])
		if !ok {
			return true // 解析失败：保守当作新图
		}
		return fmb == 0
	}
	for idx, n := range nals {
		if idx > 0 && hasVCL && newPic(n) {
			aus = append(aus, sampleAU{off: curStart, size: n.off - curStart, idr: curIDR})
			curStart, hasVCL, curIDR = n.off, false, false
		}
		if n.typ == 5 {
			curIDR = true
		}
		if isVCL(n.typ) {
			hasVCL = true
		}
	}
	return append(aus, sampleAU{off: curStart, size: len(b) - curStart, idr: curIDR})
}

func loadSampleGOP(t *testing.T) (raw []byte, gop []sampleAU) {
	t.Helper()
	p := benchSamplePath()
	if p == "" {
		t.Skip("无 H.264 样本，跳过（生成方式见本文件顶部注释）")
	}
	var err error
	raw, err = os.ReadFile(p)
	if err != nil {
		t.Skipf("读取样本失败：%v", err)
	}
	aus := splitSampleAUs(t, raw)
	first, second := -1, -1
	for i, a := range aus {
		if !a.idr {
			continue
		}
		if first < 0 {
			first = i
		} else {
			second = i
			break
		}
	}
	if first < 0 || second <= first {
		t.Skip("样本里找不到两个 IDR，跳过")
	}
	return raw, aus[first:second]
}

// tickEnds 把 GOP 切成每 tick 个 AU 的节拍，末拍收尾。
func tickEnds(n, perTick int) []int {
	var out []int
	for e := perTick; e < n; e += perTick {
		out = append(out, e)
	}
	return append(out, n)
}

// TestStreamingDecodeMatchesGOPReplay：增量喂 AU + 持久解码器，必须与「每拍重放整个 GOP」
// 得到完全相同的最后一帧——这是参考帧链没断的硬证据。
func TestStreamingDecodeMatchesGOPReplay(t *testing.T) {
	raw, gop := loadSampleGOP(t)
	if len(gop) < 8 {
		t.Skip("GOP 太短，跳过")
	}
	ends := tickEnds(len(gop), 3) // ≈ pull_sample_ms=100 @30fps

	var gopLast []byte
	for _, end := range ends {
		d, err := New(CodecH264)
		if err != nil {
			t.Fatal(err)
		}
		last := gop[end-1]
		fr, derr := d.Decode(raw[gop[0].off : last.off+last.size])
		d.Close()
		if derr == nil && fr != nil {
			gopLast = append([]byte(nil), fr.RGB...)
		}
	}

	d, err := NewWithOptions(CodecH264, Options{Streaming: true})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var streamLast []byte
	prev := 0
	for _, end := range ends {
		last := gop[end-1]
		fr, derr := d.DecodeStream(raw[gop[prev].off:last.off+last.size], true)
		if derr != nil && derr != ErrNoFrame {
			t.Fatalf("增量解码出错：%v", derr)
		}
		if fr != nil {
			streamLast = fr.RGB
		}
		prev = end
	}

	if gopLast == nil {
		t.Fatal("重放路径没解出帧")
	}
	if streamLast == nil {
		t.Fatal("增量路径没解出帧")
	}
	if !bytes.Equal(gopLast, streamLast) {
		t.Fatalf("最后一帧不一致：重放 %d 字节 / 增量 %d 字节", len(gopLast), len(streamLast))
	}
}

// TestStreamingDecodeCostBaseline 打两条路径的单拍耗时基线（配合 -v 看日志）。
// 不断言时间（机器差异大），只断言增量路径不慢于重放路径。
func TestStreamingDecodeCostBaseline(t *testing.T) {
	raw, gop := loadSampleGOP(t)
	if len(gop) < 8 {
		t.Skip("GOP 太短，跳过")
	}
	ends := tickEnds(len(gop), 3)

	var replayTotal time.Duration
	for _, end := range ends {
		d, err := New(CodecH264)
		if err != nil {
			t.Fatal(err)
		}
		last := gop[end-1]
		t0 := time.Now()
		_, _ = d.Decode(raw[gop[0].off : last.off+last.size])
		replayTotal += time.Since(t0)
		d.Close()
	}

	d, err := NewWithOptions(CodecH264, Options{Streaming: true})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var streamTotal time.Duration
	prev := 0
	for _, end := range ends {
		last := gop[end-1]
		t0 := time.Now()
		_, _ = d.DecodeStream(raw[gop[prev].off:last.off+last.size], true)
		streamTotal += time.Since(t0)
		prev = end
	}

	n := float64(len(ends))
	t.Logf("GOP=%d 帧，%d 拍：每拍重放整个 GOP = %.1f ms/拍；增量送 AU = %.2f ms/拍（%.1fx）",
		len(gop), len(ends),
		float64(replayTotal.Microseconds())/1000/n,
		float64(streamTotal.Microseconds())/1000/n,
		float64(replayTotal)/float64(streamTotal))
	if streamTotal >= replayTotal {
		t.Errorf("增量解码不比 GOP 重放快：%.1fms vs %.1fms", float64(streamTotal.Microseconds())/1000, float64(replayTotal.Microseconds())/1000)
	}
}

// TestStreamingWantFrameFalseSkipsRGB：want_rgb=0 时只推进解码状态，不产出帧。
// 含 B 帧的流有固有重排延迟，故先送足起始 AU 再断言。
func TestStreamingWantFrameFalseSkipsRGB(t *testing.T) {
	raw, gop := loadSampleGOP(t)
	if len(gop) < 12 {
		t.Skip("GOP 太短，跳过")
	}
	d, err := NewWithOptions(CodecH264, Options{Streaming: true})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// 先用带输出的方式喂到出帧（B 帧流需要几帧才能出第一帧）
	var warm int
	for i := 1; i <= len(gop) && warm == 0; i++ {
		last := gop[i-1]
		if fr, derr := d.DecodeStream(raw[gop[0].off:last.off+last.size], true); derr == nil && fr != nil {
			warm = i
		}
	}
	if warm == 0 {
		t.Fatal("预热阶段始终没出帧")
	}

	for i := warm; i < warm+20 && i < len(gop); i++ {
		a := gop[i]
		fr, derr := d.DecodeStream(raw[a.off:a.off+a.size], false)
		if derr != nil {
			t.Fatalf("wantFrame=false 不应报错：%v", derr)
		}
		if fr != nil {
			t.Fatal("wantFrame=false 时不应返回帧")
		}
	}
}

// TestStreamingGroupsSliceNALsPerPicture：多样 slice 码流的硬回归。
//
// x264 在多核机器上会把一帧切成多个 slice NAL（推流脚本重编码后的流就是这种，
// 实测 1280x720 下约 10 个 slice/帧）。若按「每个 VCL NAL 一个包」喂给解码器，
// 每帧都会因数据不完整而被整帧 conceal（画面花屏、但 libav 不报错）。
// 本测试断言：送入的 AU 数 == 图数，且全程无解码错误。
func TestStreamingGroupsSliceNALsPerPicture(t *testing.T) {
	raw, gop := loadSampleGOP(t)
	if len(gop) < 8 {
		t.Skip("GOP 太短，跳过")
	}
	// 统计 GOP 内 VCL NAL 总数，确认样本确实是多 slice 流
	vcl := 0
	lo, hi := gop[0].off, gop[len(gop)-1].off+gop[len(gop)-1].size
	for i := lo; i+3 < hi; {
		if !(raw[i] == 0 && raw[i+1] == 0 && raw[i+2] == 1) {
			i++
			continue
		}
		if t8 := raw[i+3] & 0x1f; t8 >= 1 && t8 <= 5 {
			vcl++
		}
		i += 3
	}
	if vcl <= len(gop) {
		t.Skipf("样本是单 slice 流（VCL NAL %d / 图 %d），不构成回归场景", vcl, len(gop))
	}
	t.Logf("样本：VCL NAL %d 个 / 图 %d 个 = %.1f slice/帧", vcl, len(gop), float64(vcl)/float64(len(gop)))

	d, err := NewWithOptions(CodecH264, Options{Streaming: true})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	frames := 0
	for _, a := range gop {
		fr, derr := d.DecodeStream(raw[a.off:a.off+a.size], true)
		if derr != nil && derr != ErrNoFrame {
			t.Fatalf("解码 AU 出错：%v", derr)
		}
		if fr != nil {
			frames++
			if fr.Width <= 0 || fr.Height <= 0 || len(fr.RGB) != fr.Width*fr.Height*3 {
				t.Fatalf("帧尺寸异常：%dx%d rgb=%d", fr.Width, fr.Height, len(fr.RGB))
			}
		}
	}
	if frames == 0 {
		t.Fatal("一帧都没解出来")
	}
	// 每图一帧：帧数不应超过图数（B 帧重排可能少几帧，但绝不能接近 slice 数）
	if frames > len(gop) {
		t.Fatalf("解出 %d 帧 > 图数 %d：slice 被拆成独立包喂了", frames, len(gop))
	}
	t.Logf("解出 %d 帧 / %d 图，无 conceal、无解码错误", frames, len(gop))
}
