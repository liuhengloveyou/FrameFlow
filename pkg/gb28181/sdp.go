package gb28181

import (
	"fmt"
	"strings"
	"time"
)

const (
	TransportUDP        = "udp"
	TransportTCPPassive = "tcp-passive"
	TransportTCPActive  = "tcp-active"
	TransportAuto       = "auto"

	// StreamMain / StreamSub are live stream profiles for INVITE SDP.
	StreamMain = "main"
	StreamSub  = "sub"
)

// NormalizeMediaTransport maps UI/URL aliases to canonical values.
// auto → fallback (empty uses gateway default / udp at dial time).
func NormalizeMediaTransport(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case TransportTCPPassive, "tcp", "passive":
		return TransportTCPPassive
	case TransportTCPActive, "active":
		return TransportTCPActive
	case TransportUDP:
		return TransportUDP
	case TransportAuto, "":
		return TransportAuto
	default:
		return TransportUDP
	}
}

// InviteSDP builds SDP for a live Play INVITE (PS over RTP).
// streamProfile: "main" | "sub" (empty defaults to sub for preview bandwidth).
func InviteSDP(mediaIP string, mediaPort uint16, ssrc string, transport string, streamProfile string) string {
	return InviteSDPWithAttr(mediaIP, mediaPort, ssrc, transport, streamProfile, nil)
}

// InvitePlaybackSDP builds SDP for a Playback INVITE with time range.
func InvitePlaybackSDP(mediaIP string, mediaPort uint16, ssrc, transport, channelID string, start, end time.Time) string {
	y := ssrc
	if y == "" {
		y = "0100000001"
	}
	var sb strings.Builder
	sb.WriteString("v=0\r\n")
	sb.WriteString(fmt.Sprintf("o=%s 0 0 IN IP4 %s\r\n", y, mediaIP))
	sb.WriteString("s=Playback\r\n")
	sb.WriteString(fmt.Sprintf("u=%s:0\r\n", channelID))
	sb.WriteString(fmt.Sprintf("c=IN IP4 %s\r\n", mediaIP))
	sb.WriteString(fmt.Sprintf("t=%d %d\r\n", start.Unix(), end.Unix()))
	switch strings.ToLower(transport) {
	case TransportTCPPassive, "tcp":
		sb.WriteString(fmt.Sprintf("m=video %d TCP/RTP/AVP 96\r\n", mediaPort))
		sb.WriteString("a=setup:passive\r\n")
		sb.WriteString("a=connection:new\r\n")
	case TransportTCPActive, "active":
		sb.WriteString(fmt.Sprintf("m=video %d TCP/RTP/AVP 96\r\n", mediaPort))
		sb.WriteString("a=setup:active\r\n")
		sb.WriteString("a=connection:new\r\n")
	default:
		sb.WriteString(fmt.Sprintf("m=video %d RTP/AVP 96\r\n", mediaPort))
	}
	sb.WriteString("a=recvonly\r\n")
	sb.WriteString("a=rtpmap:96 PS/90000\r\n")
	sb.WriteString(fmt.Sprintf("y=%s\r\n", y))
	return sb.String()
}

// InviteSDPWithAttr allows custom SDP attribute bodies (without "a=" prefix).
// If streamAttrs is empty, vendor-compatible defaults from StreamSDPAttrs are used.
func InviteSDPWithAttr(mediaIP string, mediaPort uint16, ssrc, transport, streamProfile string, streamAttrs []string) string {
	y := ssrc
	if y == "" {
		y = "0100000001"
	}
	profile := NormalizeStreamProfile(streamProfile)
	attrs := streamAttrs
	if len(attrs) == 0 {
		attrs = StreamSDPAttrs(profile)
	}

	var sb strings.Builder
	sb.WriteString("v=0\r\n")
	sb.WriteString(fmt.Sprintf("o=%s 0 0 IN IP4 %s\r\n", y, mediaIP))
	sb.WriteString("s=Play\r\n")
	sb.WriteString(fmt.Sprintf("c=IN IP4 %s\r\n", mediaIP))
	sb.WriteString("t=0 0\r\n")
	switch strings.ToLower(transport) {
	case TransportTCPPassive, "tcp":
		sb.WriteString(fmt.Sprintf("m=video %d TCP/RTP/AVP 96\r\n", mediaPort))
		sb.WriteString("a=setup:passive\r\n")
		sb.WriteString("a=connection:new\r\n")
	case TransportTCPActive, "active":
		sb.WriteString(fmt.Sprintf("m=video %d TCP/RTP/AVP 96\r\n", mediaPort))
		sb.WriteString("a=setup:active\r\n")
		sb.WriteString("a=connection:new\r\n")
	default:
		sb.WriteString(fmt.Sprintf("m=video %d RTP/AVP 96\r\n", mediaPort))
	}
	sb.WriteString("a=recvonly\r\n")
	for _, attr := range attrs {
		attr = strings.TrimSpace(attr)
		if attr == "" {
			continue
		}
		sb.WriteString("a=" + attr + "\r\n")
	}
	sb.WriteString("a=rtpmap:96 PS/90000\r\n")
	sb.WriteString(fmt.Sprintf("y=%s\r\n", y))
	return sb.String()
}

// NormalizeStreamProfile maps aliases to main|sub; empty → sub.
func NormalizeStreamProfile(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StreamMain, "0", "hd", "high":
		return StreamMain
	case StreamSub, "1", "sd", "low", "":
		return StreamSub
	default:
		return StreamSub
	}
}

// DefaultStreamSDPAttr returns a single a= attribute body (legacy helper).
// Prefer StreamSDPAttrs for multi-vendor INVITE.
func DefaultStreamSDPAttr(profile string) string {
	attrs := StreamSDPAttrs(profile)
	if len(attrs) == 0 {
		return ""
	}
	return attrs[0]
}

// StreamSDPAttrs returns multi-vendor SDP a= bodies for main/sub selection.
// Cameras ignore unknown attributes; sending all common forms maximizes compatibility:
//   - stream:N / stream:MAIN|SUB — generic / some NVR
//   - streamnumber:N — Hikvision
//   - streamprofile:N — Dahua
//   - streamMode:main|sub — Mercury / TP-Link
// N: 0 = main, 1 = sub.
func StreamSDPAttrs(profile string) []string {
	n, mode, name := "1", "sub", "SUB"
	if NormalizeStreamProfile(profile) == StreamMain {
		n, mode, name = "0", "main", "MAIN"
	}
	return []string{
		"stream:" + n,
		"stream:" + name,
		"streamnumber:" + n,
		"streamprofile:" + n,
		"streamMode:" + mode,
	}
}

// SubjectHeader builds GB28181 Subject header: channelID:ssrc,deviceID:serial
func SubjectHeader(channelID, ssrc, deviceID, serverID string) string {
	return fmt.Sprintf("%s:%s,%s:%s", channelID, ssrc, deviceID, serverID)
}

// ParseAnswerMedia extracts media IP and port from an INVITE 200 OK SDP body.
func ParseAnswerMedia(sdp []byte) (ip string, port int, ok bool) {
	lines := strings.Split(string(sdp), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "c=") {
			// c=IN IP4 1.2.3.4
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				ip = parts[len(parts)-1]
			}
		}
		if strings.HasPrefix(line, "m=video") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var p int
				if _, err := fmt.Sscanf(parts[1], "%d", &p); err == nil && p > 0 {
					port = p
				}
			}
		}
	}
	return ip, port, ip != "" && port > 0
}

// MakeSSRC builds a simple decimal SSRC string used in y= and Subject.
func MakeSSRC(seq uint32) string {
	return fmt.Sprintf("%010d", uint64(seq)%10000000000)
}
