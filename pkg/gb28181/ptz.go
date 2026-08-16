package gb28181

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// PTZ command names mapped to GB28181 PTZCmd first control byte semantics.
// Encoding: A5 + version/check + address + command + data1 + data2 + check
const (
	PTZStop      = "stop"
	PTZRight     = "right"
	PTZLeft      = "left"
	PTZUp        = "up"
	PTZDown      = "down"
	PTZZoomIn    = "zoomin"
	PTZZoomOut   = "zoomout"
)

// BuildPTZCmd encodes a standard 8-byte PTZCmd hex string.
// speed is 1..255; 0 treated as 64.
func BuildPTZCmd(cmd string, speed byte) (string, error) {
	if speed == 0 {
		speed = 64
	}
	var control byte
	var data1, data2 byte
	switch strings.ToLower(cmd) {
	case PTZStop, "idle":
		control = 0x00
	case PTZRight:
		control = 0x01
		data1 = speed
	case PTZLeft:
		control = 0x02
		data1 = speed
	case PTZDown:
		control = 0x04
		data2 = speed
	case PTZUp:
		control = 0x08
		data2 = speed
	case PTZZoomIn, "zoom_in":
		control = 0x10
		data1 = speed // zoom speed in high nibble typically; keep simple
	case PTZZoomOut, "zoom_out":
		control = 0x20
		data1 = speed
	default:
		return "", fmt.Errorf("unsupported ptz cmd %q", cmd)
	}

	addr := byte(1)
	buf := []byte{0xA5, 0x0F, addr, control, data1, data2, 0x00}
	var sum int
	for i := 0; i < 6; i++ {
		sum += int(buf[i])
	}
	buf[6] = byte(sum % 256)
	// some devices expect 8 bytes with trailing 00
	out := make([]byte, 8)
	copy(out, buf)
	return strings.ToUpper(hex.EncodeToString(out)), nil
}
