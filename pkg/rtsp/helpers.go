package rtsp

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/liuhengloveyou/FrameFlow/pkg/aac"
	"github.com/liuhengloveyou/FrameFlow/pkg/core"
	"github.com/pion/rtcp"
	"github.com/pion/sdp/v3"
)

type RTCP struct {
	Channel byte
	Header  rtcp.Header
	Packets []rtcp.Packet
}

const sdpHeader = `v=0
o=- 0 0 IN IP4 0.0.0.0
s=-
t=0 0`

func UnmarshalSDP(rawSDP []byte) ([]*core.Media, error) {
	sd := &sdp.SessionDescription{}
	if err := sd.Unmarshal(rawSDP); err != nil {
		// fix multiple `s=` https://github.com/AlexxIT/WebRTC/issues/417
		rawSDP = regexp.MustCompile("\ns=[^\n]+").ReplaceAll(rawSDP, nil)

		// fix broken `c=` https://github.com/liuhengloveyou/FrameFlow/issues/1426
		rawSDP = regexp.MustCompile("\nc=[^\n]+").ReplaceAll(rawSDP, nil)

		// fix SDP header for some cameras
		if i := bytes.Index(rawSDP, []byte("\nm=")); i > 0 {
			rawSDP = append([]byte(sdpHeader), rawSDP[i:]...)
		}

		// Fix invalid media type (errSDPInvalidValue) caused by
		// some TP-LINK IP camera, e.g. TL-IPC44GW
		for _, b := range regexp.MustCompile("m=[^ ]+ ").FindAll(rawSDP, -1) {
			switch string(b[2 : len(b)-1]) {
			case "audio", "video", "application":
			default:
				rawSDP = bytes.Replace(rawSDP, b, []byte("m=application "), 1)
			}
		}

		if err == io.EOF {
			rawSDP = append(rawSDP, '\n')
		}

		sd = &sdp.SessionDescription{}
		err = sd.Unmarshal(rawSDP)
		if err != nil {
			return nil, err
		}
	}

	// fix buggy camera https://github.com/liuhengloveyou/FrameFlow/issues/771
	forceDirection := sd.Origin.Username == "CV-RTSPHandler"

	var medias []*core.Media

	for _, md := range sd.MediaDescriptions {
		media := core.UnmarshalMedia(md)

		// Check buggy SDP with fmtp for H264 on another track
		// https://github.com/AlexxIT/WebRTC/issues/419
		for _, codec := range media.Codecs {
			switch codec.Name {
			case core.CodecH264:
				if codec.FmtpLine == "" {
					codec.FmtpLine = findFmtpLine(codec.PayloadType, sd.MediaDescriptions)
				}
			case core.CodecH265:
				if codec.FmtpLine != "" {
					// all three parameters are needed for a valid fmtp line
					// https://github.com/liuhengloveyou/FrameFlow/pull/1588
					if !strings.Contains(codec.FmtpLine, "sprop-vps=") ||
						!strings.Contains(codec.FmtpLine, "sprop-sps=") ||
						!strings.Contains(codec.FmtpLine, "sprop-pps=") {
						codec.FmtpLine = ""
					}
				}
			case core.CodecOpus:
				// fix OPUS for some cameras https://datatracker.ietf.org/doc/html/rfc7587
				codec.ClockRate = 48000
				codec.Channels = 2
			case core.CodecAAC:
				// 从 fmtp config= 解析采样率/声道（覆盖 rtpmap；ffmpeg -c copy 的 5.1 等）
				if conf := core.Between(codec.FmtpLine, "config=", ";"); conf != "" {
					if b, err := hex.DecodeString(conf); err == nil && len(b) > 0 {
						parsed := aac.ConfigToCodec(b)
						log.Info().
							Str("config", conf).
							Uint8("rtpmap_ch", codec.Channels).
							Uint8("config_ch", parsed.Channels).
							Uint32("rate", parsed.ClockRate).
							Msg("[rtsp] aac from fmtp config")
						if parsed.ClockRate > 0 {
							codec.ClockRate = parsed.ClockRate
						}
						// ASC channelConfiguration=0 表示随后带 PCE（多为 5.1）；
						// 不要用 0 覆盖 rtpmap 已解析的声道数。
						if parsed.Channels > 0 {
							codec.Channels = parsed.Channels
						} else if len(b) > 2 && codec.Channels == 0 {
							log.Warn().
								Str("config", conf).
								Int("config_len", len(b)).
								Msg("[rtsp] aac ASC channels=0 (PCE); treat as multi-channel")
							codec.Channels = 255 // 标记为 MSE 不可用
						}
						if codec.FmtpLine == "" {
							codec.FmtpLine = parsed.FmtpLine
						}
					} else {
						_, decErr := hex.DecodeString(conf)
						log.Warn().Str("config", conf).Err(decErr).Msg("[rtsp] aac config decode failed")
					}
				} else {
					log.Info().
						Uint8("channels", codec.Channels).
						Uint32("rate", codec.ClockRate).
						Str("fmtp", codec.FmtpLine).
						Msg("[rtsp] aac without config= in fmtp")
				}
			}
		}

		if media.Direction == "" || forceDirection {
			media.Direction = core.DirectionRecvonly
		}

		medias = append(medias, media)
	}

	return medias, nil
}

func findFmtpLine(payloadType uint8, descriptions []*sdp.MediaDescription) string {
	s := strconv.Itoa(int(payloadType))
	for _, md := range descriptions {
		codec := core.UnmarshalCodec(md, s)
		if codec.FmtpLine != "" {
			return codec.FmtpLine
		}
	}
	return ""
}

// urlParse fix bugs:
// 1. Content-Base: rtsp://::ffff:192.168.1.123/onvif/profile.1/
// 2. Content-Base: rtsp://rtsp://turret2-cam.lan:554/stream1/
// 3. Content-Base: 192.168.253.220:1935/
func urlParse(rawURL string) (*url.URL, error) {
	// fix https://github.com/liuhengloveyou/FrameFlow/issues/830
	if strings.HasPrefix(rawURL, "rtsp://rtsp://") {
		rawURL = rawURL[7:]
	}

	// fix https://github.com/liuhengloveyou/FrameFlow/issues/1852
	if !strings.Contains(rawURL, "://") {
		rawURL = "rtsp://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil && strings.HasSuffix(err.Error(), "after host") {
		if i := indexN(rawURL, '/', 3); i > 0 {
			return urlParse(rawURL[:i] + ":" + rawURL[i:])
		}
	}

	return u, err
}

func indexN(s string, c byte, n int) int {
	var offset int
	for {
		i := strings.IndexByte(s[offset:], c)
		if i < 0 {
			break
		}
		if n--; n == 0 {
			return offset + i
		}
		offset += i + 1
	}
	return -1
}
