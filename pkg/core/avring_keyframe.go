package core

import "encoding/binary"

// packetIsIDR reports whether payload is a keyframe for the given codec.
// AVCC (RAW) uses length-prefixed NALUs; RTP uses RFC6184/7798 headers.
func packetIsIDR(codecName string, isRTP bool, payload []byte) bool {
	switch codecName {
	case CodecH264:
		if isRTP {
			return isH264RTPKeyframe(payload)
		}
		return isH264AVCCKeyframe(payload)
	case CodecH265:
		if isRTP {
			return isH265RTPKeyframe(payload)
		}
		return isH265AVCCKeyframe(payload)
	default:
		return false
	}
}

func isH264AVCCKeyframe(b []byte) bool {
	for len(b) >= 5 {
		nalType := b[4] & 0x1F
		switch nalType {
		case 1: // P
			return false
		case 5: // IDR
			return true
		}
		size := int(binary.BigEndian.Uint32(b)) + 4
		if size <= 4 || size > len(b) {
			return false
		}
		if size == len(b) {
			return false
		}
		b = b[size:]
	}
	return false
}

func isH265AVCCKeyframe(b []byte) bool {
	for len(b) >= 5 {
		nalType := (b[4] >> 1) & 0x3F
		switch nalType {
		case 1: // P
			return false
		case 19, 20, 21: // IDR / CRA
			return true
		}
		size := int(binary.BigEndian.Uint32(b)) + 4
		if size <= 4 || size > len(b) {
			return false
		}
		if size == len(b) {
			return false
		}
		b = b[size:]
	}
	return false
}

func isH264RTPKeyframe(payload []byte) bool {
	if len(payload) < 1 {
		return false
	}
	nalType := payload[0] & 0x1F
	switch nalType {
	case 5:
		return true
	case 24: // STAP-A
		return stapAHasH264IDR(payload[1:])
	case 28: // FU-A
		if len(payload) < 2 {
			return false
		}
		start := payload[1]&0x80 != 0
		return start && (payload[1]&0x1F) == 5
	}
	return false
}

func stapAHasH264IDR(b []byte) bool {
	for len(b) >= 2 {
		n := int(binary.BigEndian.Uint16(b))
		b = b[2:]
		if n <= 0 || n > len(b) {
			return false
		}
		if b[0]&0x1F == 5 {
			return true
		}
		b = b[n:]
	}
	return false
}

func isH265RTPKeyframe(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	nalType := (payload[0] >> 1) & 0x3F
	switch nalType {
	case 19, 20, 21:
		return true
	case 48: // AP
		return apHasH265IDR(payload[2:])
	case 49: // FU
		if len(payload) < 3 {
			return false
		}
		start := payload[2]&0x80 != 0
		fuType := payload[2] & 0x3F
		return start && (fuType == 19 || fuType == 20 || fuType == 21)
	}
	return false
}

func apHasH265IDR(b []byte) bool {
	for len(b) >= 2 {
		n := int(binary.BigEndian.Uint16(b))
		b = b[2:]
		if n < 2 || n > len(b) {
			return false
		}
		t := (b[0] >> 1) & 0x3F
		if t == 19 || t == 20 || t == 21 {
			return true
		}
		b = b[n:]
	}
	return false
}
