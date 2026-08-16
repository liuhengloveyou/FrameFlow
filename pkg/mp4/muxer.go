package mp4

import (
	"encoding/hex"

	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/liuhengloveyou/FrameFlow/pkg/h264"
	"github.com/liuhengloveyou/FrameFlow/pkg/h265"
	"github.com/liuhengloveyou/FrameFlow/pkg/iso"
	"github.com/pion/rtp"
)

type Muxer struct {
	index  uint32
	dts    []uint64
	pts    []uint32
	codecs []*core.Codec

	// 无显式 CTS 时（如 RTSP 推流），按 PTS 估计 DTS（对齐 monibuca DTSEstimator）
	dtsEst  []core.DTSEstimator
	estInit []bool
	estDTS  []uint32

	// 诊断：前若干视频样本打 PTS/DTS/CTS
	diagSamples int
}

func (m *Muxer) AddTrack(codec *core.Codec) {
	m.dts = append(m.dts, 0)
	m.pts = append(m.pts, 0)
	m.codecs = append(m.codecs, codec)
	m.dtsEst = append(m.dtsEst, core.DTSEstimator{})
	m.estInit = append(m.estInit, false)
	m.estDTS = append(m.estDTS, 0)
}

func (m *Muxer) GetInit() ([]byte, error) {
	mv := iso.NewMovie(1024)
	mv.WriteFileType()

	mv.StartAtom(iso.Moov)
	mv.WriteMovieHeader()

	for i, codec := range m.codecs {
		switch codec.Name {
		case core.CodecH264:
			sps, pps := h264.GetParameterSet(codec.FmtpLine)
			// some dummy SPS and PPS not a problem for MP4, but problem for HLS :(
			if len(sps) == 0 {
				sps = []byte{0x67, 0x42, 0x00, 0x0a, 0xf8, 0x41, 0xa2}
			}
			if len(pps) == 0 {
				pps = []byte{0x68, 0xce, 0x38, 0x80}
			}

			var width, height uint16
			if s := h264.DecodeSPS(sps); s != nil {
				width = s.Width()
				height = s.Height()
			} else {
				width = 1920
				height = 1080
			}

			mv.WriteVideoTrack(
				uint32(i+1), codec.Name, codec.ClockRate, width, height, h264.EncodeConfig(sps, pps),
			)

		case core.CodecH265:
			vps, sps, pps := h265.GetParameterSet(codec.FmtpLine)
			// some dummy SPS and PPS not a problem
			if len(vps) == 0 {
				vps = []byte{0x40, 0x01, 0x0c, 0x01, 0xff, 0xff, 0x01, 0x40, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x99, 0xac, 0x09}
			}
			if len(sps) == 0 {
				sps = []byte{0x42, 0x01, 0x01, 0x01, 0x40, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x99, 0xa0, 0x01, 0x40, 0x20, 0x05, 0xa1, 0xfe, 0x5a, 0xee, 0x46, 0xc1, 0xae, 0x55, 0x04}
			}
			if len(pps) == 0 {
				pps = []byte{0x44, 0x01, 0xc0, 0x73, 0xc0, 0x4c, 0x90}
			}

			var width, height uint16
			if s := h265.DecodeSPS(sps); s != nil {
				width = s.Width()
				height = s.Height()
			} else {
				width = 1920
				height = 1080
			}

			mv.WriteVideoTrack(
				uint32(i+1), codec.Name, codec.ClockRate, width, height, h265.EncodeConfig(vps, sps, pps),
			)

		case core.CodecAAC:
			s := core.Between(codec.FmtpLine, "config=", ";")
			b, err := hex.DecodeString(s)
			if err != nil {
				return nil, err
			}

			mv.WriteAudioTrack(
				uint32(i+1), codec.Name, codec.ClockRate, uint16(codec.Channels), b,
			)

		case core.CodecOpus, core.CodecMP3, core.CodecPCMA, core.CodecPCMU, core.CodecFLAC:
			mv.WriteAudioTrack(
				uint32(i+1), codec.Name, codec.ClockRate, uint16(codec.Channels), nil,
			)
		}
	}

	mv.StartAtom(iso.MoovMvex)
	for i := range m.codecs {
		mv.WriteTrackExtend(uint32(i + 1))
	}
	mv.EndAtom() // MVEX

	mv.EndAtom() // MOOV

	init := mv.Bytes()
	names := make([]string, len(m.codecs))
	for i, c := range m.codecs {
		names[i] = c.String()
	}
	log.Info().Strs("tracks", names).Int("bytes", len(init)).Msg("[mp4] init segment")

	return init, nil
}

func (m *Muxer) Reset() {
	m.index = 0
	for i := range m.dts {
		m.dts[i] = 0
		m.pts[i] = 0
		m.dtsEst[i] = core.DTSEstimator{}
		m.estInit[i] = false
		m.estDTS[i] = 0
	}
}

func (m *Muxer) GetPayload(trackID byte, packet *rtp.Packet) []byte {
	codec := m.codecs[trackID]

	m.index++

	var (
		duration uint32
		cts      int32
	)

	isVideo := codec.Name == core.CodecH264 || codec.Name == core.CodecH265
	var estDTS uint32
	usedEstimator := false
	// FLV/mpegts 等路径已把 CTS 放进 ExtensionProfile，Timestamp 为 DTS。
	// RTSP 等路径 Timestamp 为 PTS 且无 CTS → 用 DTSEstimator（对齐 monibuca）。
	if isVideo && packet.ExtensionProfile == 0 {
		pts := packet.Timestamp
		estDTS = m.dtsEst[trackID].Feed(pts)
		cts = int32(pts - estDTS) // uint32 模运算 → 有符号 CTS
		if m.estInit[trackID] {
			duration = estDTS - m.estDTS[trackID]
		}
		m.estDTS[trackID] = estDTS
		m.estInit[trackID] = true
		m.pts[trackID] = pts
		usedEstimator = true
	} else {
		duration = packet.Timestamp - m.pts[trackID]
		m.pts[trackID] = packet.Timestamp
		if packet.ExtensionProfile != 0 {
			cts = int32(packet.ExtensionProfile)
		}
	}

	// flags important for Apple Finder video preview
	var flags uint32

	switch codec.Name {
	case core.CodecH264:
		if h264.IsKeyframe(packet.Payload) {
			flags = iso.SampleVideoIFrame
		} else {
			flags = iso.SampleVideoNonIFrame
		}
	case core.CodecH265:
		if h265.IsKeyframe(packet.Payload) {
			flags = iso.SampleVideoIFrame
		} else {
			flags = iso.SampleVideoNonIFrame
		}
	case core.CodecAAC:
		duration = 1024         // important for Apple Finder and QuickTime
		flags = iso.SampleAudio // not important?
	default:
		flags = iso.SampleAudio // important for FLAC on Android Telegram
	}

	// minumum duration important for MSE in Apple Safari
	if duration == 0 || duration > codec.ClockRate {
		duration = codec.ClockRate/1000 + 1
		if !(isVideo && packet.ExtensionProfile == 0) {
			m.pts[trackID] += duration
		}
	}

	size := len(packet.Payload)

	if isVideo && m.diagSamples < 12 {
		m.diagSamples++
		log.Info().
			Uint8("track", trackID).
			Str("codec", codec.Name).
			Uint32("pts", packet.Timestamp).
			Uint32("ext_cts", uint32(packet.ExtensionProfile)).
			Bool("estimator", usedEstimator).
			Uint32("est_dts", estDTS).
			Int32("cts", cts).
			Uint32("duration", duration).
			Uint64("base_dts", m.dts[trackID]).
			Int("size", size).
			Uint32("flags", flags).
			Msg("[mp4] video sample")
	}

	mv := iso.NewMovie(1024 + size)
	mv.WriteMovieFragment(
		m.index, uint32(trackID+1), duration, uint32(size), flags, m.dts[trackID], cts,
	)
	mv.WriteData(packet.Payload)

	m.dts[trackID] += uint64(duration)

	return mv.Bytes()
}
