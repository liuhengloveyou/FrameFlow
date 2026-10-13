package magic

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/liuhengloveyou/FrameFlow/pkg/h264/annexb"
	"github.com/pion/rtp"
)

// avccAU 拼一个 AVCC 访问单元：[4字节长度][NAL 头 + 负载]...，nalTypes 为各 NAL 的 (type|0x60)。
func avccAU(nalTypes ...byte) []byte {
	var out []byte
	for i, t := range nalTypes {
		nal := []byte{t, byte(i + 1), byte(i + 2), byte(i + 3)}
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(nal)))
		out = append(out, l[:]...)
		out = append(out, nal...)
	}
	return out
}

// pkt 构造只带时间戳的 RTP 包（consumer 只用到 Timestamp 与 Payload）。
func pkt(ts uint32) *rtp.Packet { return &rtp.Packet{Header: rtp.Header{Timestamp: ts}} }

func annexB(t *testing.T, au []byte) []byte {
	t.Helper()
	b := annexb.DecodeAVCC(au, true)
	if len(b) == 0 {
		t.Fatalf("DecodeAVCC 返回空：% x", au)
	}
	return b
}

// 增量模式必须只推送「新到的 AU」：第一份是 IDR 本身，不能像自包含模式那样把整个 GOP 重发。
func TestIncrementalSampleConsumerEmitsOnlyNewAUs(t *testing.T) {
	c := NewIncrementalSampleConsumer(0, 1<<20)
	idr := avccAU(0x67, 0x68, 0x65) // SPS + PPS + IDR
	p1 := avccAU(0x41)
	p2 := avccAU(0x41)

	wantIDR := annexB(t, idr)
	c.onH264AU(pkt(1), idr)
	got, err := c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	if !bytes.Equal(got, wantIDR) {
		t.Fatalf("第一份样本应只含 IDR AU，len(got)=%d want=%d", len(got), len(wantIDR))
	}

	c.onH264AU(pkt(2), p1)
	got, err = c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	if want := annexB(t, p1); !bytes.Equal(got, want) {
		t.Fatalf("第二份样本应为 p1 增量，len(got)=%d want=%d", len(got), len(want))
	}

	// 累计字节必须与输入完全一致（不重不漏），这是解码器参考帧不断链的前提。
	want := annexB(t, p2)
	c.onH264AU(pkt(3), p2)
	got, err = c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("第三份样本应为 p2 增量，len(got)=%d want=%d", len(got), len(want))
	}
}

// 消费端没取走上一份增量时，后续码流必须继续累积而不是被覆盖丢弃——
// 否则解码器出现参考帧缺口（花屏直到下一个 IDR）。
func TestIncrementalSampleConsumerKeepsPendingWhenBusy(t *testing.T) {
	c := NewIncrementalSampleConsumer(0, 1<<20)
	idr := avccAU(0x67, 0x68, 0x65)
	p1 := avccAU(0x41)
	p2 := avccAU(0x41)
	p3 := avccAU(0x41)

	c.onH264AU(pkt(1), idr) // 进 sampleCh
	c.onH264AU(pkt(2), p1)  // 通道忙：不推送，累积
	c.onH264AU(pkt(3), p2)  // 仍忙：继续累积

	got, err := c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	if !bytes.Equal(got, annexB(t, idr)) {
		t.Fatalf("第一份样本应为 IDR")
	}

	c.onH264AU(pkt(4), p3)
	got, err = c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	want := append(annexB(t, p1), annexB(t, p2)...)
	want = append(want, annexB(t, p3)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("忙时的增量必须累积补发：len(got)=%d want=%d（丢帧会导致解码缺口）", len(got), len(want))
	}
}

// 解码器重置后：RequestResync 应丢弃未送出的增量（含已压进通道的旧样本），
// 并等下一个 IDR 作为自包含起点。
func TestIncrementalSampleConsumerResyncWaitsForIDR(t *testing.T) {
	c := NewIncrementalSampleConsumer(0, 1<<20)
	idr := avccAU(0x67, 0x68, 0x65)

	c.onH264AU(pkt(1), idr) // 进 sampleCh
	c.onH264AU(pkt(2), avccAU(0x41))
	c.RequestResync() // 解码器已重置：通道里的旧样本与未送增量都不可用

	// resync 后第一个 P 帧应被丢弃（解码器无参考帧）
	c.onH264AU(pkt(3), avccAU(0x41))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if b, err := c.WaitSample(ctx); err == nil {
		t.Fatalf("resync 后不应推送旧增量/P 帧，却收到 %d 字节", len(b))
	}

	// 下一个 IDR 应被推送，且是从干净起点开始
	c.onH264AU(pkt(4), idr)
	got, err := c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	if want := annexB(t, idr); !bytes.Equal(got, want) {
		t.Fatalf("resync 后应以新 IDR 重新开始：len(got)=%d want=%d", len(got), len(want))
	}
}

// 自包含模式（默认）行为不变：每份样本都是「自 IDR 起的完整片段」。
func TestSelfContainedSampleConsumerStillReplaysGOP(t *testing.T) {
	c := NewIntervalSampleConsumer(0, 1<<20)
	idr := avccAU(0x67, 0x68, 0x65)
	p1 := avccAU(0x41)

	c.onH264AU(pkt(1), idr)
	if _, err := c.WaitSample(context.Background()); err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	c.onH264AU(pkt(2), p1)
	got, err := c.WaitSample(context.Background())
	if err != nil {
		t.Fatalf("WaitSample: %v", err)
	}
	want := append(annexB(t, idr), annexB(t, p1)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("自包含模式应重放整个 GOP：len(got)=%d want=%d", len(got), len(want))
	}
}
