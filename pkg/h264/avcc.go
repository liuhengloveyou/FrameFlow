// Package h264 - AVCC format related functions
package h264

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/pion/rtp"
)

func RepairAVCC(codec *core.Codec, handler core.HandlerFunc) core.HandlerFunc {
	sps, pps := GetParameterSet(codec.FmtpLine)
	ps := JoinNALU(sps, pps)

	return func(packet *rtp.Packet) {
		if len(packet.Payload) < 5 {
			clone := *packet
			clone.Version = RTPPacketVersionAVC
			handler(&clone)
			return
		}

		payload := packet.Payload
		changed := false

		// this can happen for FLV from FFmpeg
		if NALUType(payload) == NALUTypeSEI {
			size := int(binary.BigEndian.Uint32(payload)) + 4
			if size > 0 && size <= len(payload) {
				payload = payload[size:]
				changed = true
			}
		}
		// 关键帧缺 SPS/PPS 时注入（AU 可能以 AUD/SEI 开头，不能只看首 NALU）
		if len(ps) > 0 {
			fixed := EnsureKeyframeParamSets(payload, sps, pps)
			if len(fixed) > len(payload) {
				payload = fixed
				changed = true
			}
		}

		// 同一 Receiver 会把同一 *Packet 扇出给多个 Sender；禁止就地改 Payload，
		// 否则 WebRTC / vision 等并发 consumer 会互相踩坏 NAL（表现为花屏、解不出帧）。
		// 显式 Version=0，确保下游 RTPPay 走 AVCC 分包而非把 Raw 当 RTP 直通。
		clone := *packet
		clone.Version = RTPPacketVersionAVC
		if changed {
			clone.Payload = payload
		}
		handler(&clone)
	}
}

func JoinNALU(nalus ...[]byte) (avcc []byte) {
	var i, n int

	for _, nalu := range nalus {
		if i = len(nalu); i > 0 {
			n += 4 + i
		}
	}

	avcc = make([]byte, n)

	n = 0
	for _, nal := range nalus {
		if i = len(nal); i > 0 {
			binary.BigEndian.PutUint32(avcc[n:], uint32(i))
			n += 4 + copy(avcc[n+4:], nal)
		}
	}

	return
}

func SplitNALU(avcc []byte) [][]byte {
	var nals [][]byte
	for {
		// get AVC length
		size := int(binary.BigEndian.Uint32(avcc)) + 4

		// check if multiple items in one packet
		if size < len(avcc) {
			nals = append(nals, avcc[:size])
			avcc = avcc[size:]
		} else {
			nals = append(nals, avcc)
			break
		}
	}
	return nals
}

func NALUTypes(avcc []byte) []byte {
	var types []byte
	for {
		types = append(types, NALUType(avcc))

		size := 4 + int(binary.BigEndian.Uint32(avcc))
		if size < len(avcc) {
			avcc = avcc[size:]
		} else {
			break
		}
	}
	return types
}

func AVCCToCodec(avcc []byte) *core.Codec {
	buf := bytes.NewBufferString("packetization-mode=1")

	for {
		n := len(avcc)
		if n < 4 {
			break
		}

		size := 4 + int(binary.BigEndian.Uint32(avcc))
		if n < size {
			break
		}

		switch NALUType(avcc) {
		case NALUTypeSPS:
			buf.WriteString(";profile-level-id=")
			buf.WriteString(hex.EncodeToString(avcc[5:8]))
			buf.WriteString(";sprop-parameter-sets=")
			buf.WriteString(base64.StdEncoding.EncodeToString(avcc[4:size]))
		case NALUTypePPS:
			buf.WriteString(",")
			buf.WriteString(base64.StdEncoding.EncodeToString(avcc[4:size]))
		}

		avcc = avcc[size:]
	}

	return &core.Codec{
		Name:        core.CodecH264,
		ClockRate:   90000,
		FmtpLine:    buf.String(),
		PayloadType: core.PayloadTypeRAW,
	}
}

// ExtractParameterSets 从 AVCC 访问单元中取出最后出现的 SPS/PPS（裸 NAL，无长度前缀）。
func ExtractParameterSets(avcc []byte) (sps, pps []byte) {
	for len(avcc) >= 5 {
		size := 4 + int(binary.BigEndian.Uint32(avcc))
		if size < 5 || size > len(avcc) {
			return
		}
		nal := avcc[4:size]
		switch nal[0] & 0x1F {
		case NALUTypeSPS:
			sps = append([]byte(nil), nal...)
		case NALUTypePPS:
			pps = append([]byte(nil), nal...)
		}
		if size == len(avcc) {
			return
		}
		avcc = avcc[size:]
	}
	return
}

// HasNALUType 判断 AVCC 中是否包含指定类型的 NALU。
func HasNALUType(avcc []byte, t byte) bool {
	for len(avcc) >= 5 {
		if NALUType(avcc) == t {
			return true
		}
		size := 4 + int(binary.BigEndian.Uint32(avcc))
		if size < 5 || size > len(avcc) {
			return false
		}
		if size == len(avcc) {
			return false
		}
		avcc = avcc[size:]
	}
	return false
}

// ParamSetsOnly 判断 AU 是否全是非 VCL（SPS/PPS/SEI/AUD）。
// 国标侧用其跳过「仅参数集」的 AU，不单独发布（对齐 monibuca AnnexB.Parse）。
func ParamSetsOnly(avcc []byte) bool {
	if len(avcc) < 5 {
		return true
	}
	saw := false
	for len(avcc) >= 5 {
		switch NALUType(avcc) {
		case NALUTypeSPS, NALUTypePPS, NALUTypeSEI, NALUTypeAUD:
			saw = true
		default:
			return false
		}
		size := 4 + int(binary.BigEndian.Uint32(avcc))
		if size < 5 || size > len(avcc) {
			return false
		}
		if size == len(avcc) {
			return saw
		}
		avcc = avcc[size:]
	}
	return saw
}

// EnsureKeyframeParamSets 在 IDR AU 缺少 SPS/PPS 时前置注入（AVCC 长度前缀格式）。
// 对齐 monibuca AnnexB.Parse：把缓存的参数集插入关键帧。
func EnsureKeyframeParamSets(avcc, sps, pps []byte) []byte {
	if len(avcc) < 5 || !IsKeyframe(avcc) {
		return avcc
	}
	if (HasNALUType(avcc, NALUTypeSPS) && HasNALUType(avcc, NALUTypePPS)) ||
		len(sps) == 0 || len(pps) == 0 {
		return avcc
	}
	return Join(JoinNALU(sps, pps), avcc)
}
