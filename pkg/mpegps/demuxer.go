// Package mpegps demuxes MPEG Program Stream (used by GB/T 28181 over RTP).
package mpegps

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrNeedMore = errors.New("mpegps: need more data")
	ErrSync     = errors.New("mpegps: sync lost")
)

const (
	startCodePack   = 0x000001BA
	startCodeSystem = 0x000001BB
	startCodeMap    = 0x000001BC
	startCodeEnd    = 0x000001B9
	startCodePSM    = startCodeMap
)

// Stream types (ISO/IEC 13818-1 Table 2-29 / common GB28181 usage)
const (
	StreamTypeMPEG1Video = 0x01
	StreamTypeMPEG2Video = 0x02
	StreamTypeMPEG1Audio = 0x03
	StreamTypeMPEG2Audio = 0x04
	StreamTypePrivate    = 0x06
	StreamTypeAAC        = 0x0F
	StreamTypeMPEG4      = 0x10
	StreamTypeH264       = 0x1B
	StreamTypeH265       = 0x24
	StreamTypeG711A      = 0x90 // frequently used privately for PCMA
	StreamTypeG711U      = 0x91
)

// Packet is one demuxed PES access unit (payload only).
type Packet struct {
	StreamID   byte
	StreamType byte
	PTS        uint64
	HasPTS     bool
	Payload    []byte
}

// Demuxer parses a continuous MPEG-PS byte stream.
type Demuxer struct {
	buf        bytes.Buffer
	streamType map[byte]byte // elementary stream_id -> stream_type
}

func NewDemuxer() *Demuxer {
	return &Demuxer{streamType: make(map[byte]byte)}
}

// Write appends raw PS bytes (e.g. RTP payloads concatenated).
func (d *Demuxer) Write(p []byte) (int, error) {
	return d.buf.Write(p)
}

// Next returns the next PES packet, or ErrNeedMore.
func (d *Demuxer) Next() (*Packet, error) {
	for {
		b := d.buf.Bytes()
		if len(b) < 4 {
			return nil, ErrNeedMore
		}
		idx := findStartCode(b)
		if idx < 0 {
			// keep last 3 bytes for resync
			if len(b) > 3 {
				d.buf.Next(len(b) - 3)
			}
			return nil, ErrNeedMore
		}
		if idx > 0 {
			d.buf.Next(idx)
			b = d.buf.Bytes()
		}
		if len(b) < 4 {
			return nil, ErrNeedMore
		}
		code := binary.BigEndian.Uint32(b[:4])
		switch {
		case code == startCodePack:
			n, err := parsePackHeaderLen(b)
			if err != nil {
				if errors.Is(err, ErrNeedMore) {
					return nil, ErrNeedMore
				}
				d.buf.Next(1)
				continue
			}
			d.buf.Next(n)
		case code == startCodeSystem:
			n, err := parseLengthPrefixed(b, 6)
			if err != nil {
				return nil, err
			}
			d.buf.Next(n)
		case code == startCodeMap:
			n, err := parsePSM(b, d.streamType)
			if err != nil {
				return nil, err
			}
			d.buf.Next(n)
		case code == startCodeEnd:
			d.buf.Next(4)
		case b[3] >= 0xC0 && b[3] <= 0xEF || b[3] == 0xBD || b[3] == 0xBE || b[3] == 0xBF:
			pkt, n, err := parsePES(b, d.streamType)
			if err != nil {
				return nil, err
			}
			d.buf.Next(n)
			if pkt != nil && len(pkt.Payload) > 0 {
				return pkt, nil
			}
		default:
			d.buf.Next(1)
		}
	}
}

// ReadPackets drains r until EOF, invoking fn for each PES packet.
func ReadPackets(r io.Reader, fn func(*Packet) error) error {
	d := NewDemuxer()
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			_, _ = d.Write(buf[:n])
			for {
				pkt, e := d.Next()
				if e != nil {
					if errors.Is(e, ErrNeedMore) {
						break
					}
					return e
				}
				if err := fn(pkt); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func findStartCode(b []byte) int {
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			return i
		}
	}
	return -1
}

func parsePackHeaderLen(b []byte) (int, error) {
	if len(b) < 14 {
		return 0, ErrNeedMore
	}
	// MPEG-2 pack: 00 00 01 BA + 6 SCR + mux_rate(3) + stuffing_len(1 low 3 bits)
	stuffing := int(b[13] & 0x07)
	n := 14 + stuffing
	if len(b) < n {
		return 0, ErrNeedMore
	}
	return n, nil
}

func parseLengthPrefixed(b []byte, headerFixed int) (int, error) {
	if len(b) < headerFixed {
		return 0, ErrNeedMore
	}
	length := int(binary.BigEndian.Uint16(b[4:6]))
	n := 6 + length
	if len(b) < n {
		return 0, ErrNeedMore
	}
	return n, nil
}

func parsePSM(b []byte, streamType map[byte]byte) (int, error) {
	n, err := parseLengthPrefixed(b, 6)
	if err != nil {
		return 0, err
	}
	if n < 12 {
		return n, nil
	}
	programStreamInfoLen := int(binary.BigEndian.Uint16(b[8:10]))
	pos := 10 + programStreamInfoLen
	if pos+2 > n {
		return n, nil
	}
	esMapLen := int(binary.BigEndian.Uint16(b[pos : pos+2]))
	pos += 2
	end := pos + esMapLen
	if end > n {
		end = n
	}
	for pos+4 <= end {
		st := b[pos]
		esID := b[pos+1]
		esInfoLen := int(binary.BigEndian.Uint16(b[pos+2 : pos+4]))
		streamType[esID] = st
		pos += 4 + esInfoLen
	}
	return n, nil
}

func parsePES(b []byte, streamType map[byte]byte) (*Packet, int, error) {
	if len(b) < 6 {
		return nil, 0, ErrNeedMore
	}
	streamID := b[3]
	pesPacketLength := int(binary.BigEndian.Uint16(b[4:6]))
	if pesPacketLength == 0 {
		// unbounded — uncommon in GB28181; require more until next start (handled by caller needing more)
		return nil, 0, ErrNeedMore
	}
	total := 6 + pesPacketLength
	if len(b) < total {
		return nil, 0, ErrNeedMore
	}
	if total < 9 {
		return nil, total, nil
	}
	// optional PES header
	flags := b[7]
	headerDataLen := int(b[8])
	payloadStart := 9 + headerDataLen
	if payloadStart > total {
		return nil, total, nil
	}
	pkt := &Packet{
		StreamID:   streamID,
		StreamType: guessStreamType(streamID, streamType),
		Payload:    append([]byte(nil), b[payloadStart:total]...),
	}
	if flags&0x80 != 0 && headerDataLen >= 5 {
		// PTS present
		pkt.HasPTS = true
		pkt.PTS = parsePTS(b[9:14])
	}
	return pkt, total, nil
}

func parsePTS(b []byte) uint64 {
	if len(b) < 5 {
		return 0
	}
	return (uint64(b[0]&0x0E) << 29) |
		(uint64(b[1]) << 22) |
		(uint64(b[2]&0xFE) << 14) |
		(uint64(b[3]) << 7) |
		(uint64(b[4]) >> 1)
}

func guessStreamType(streamID byte, m map[byte]byte) byte {
	if st, ok := m[streamID]; ok {
		return st
	}
	switch {
	case streamID >= 0xE0 && streamID <= 0xEF:
		return StreamTypeH264
	case streamID >= 0xC0 && streamID <= 0xDF:
		return StreamTypeG711A
	case streamID == 0xBD:
		return StreamTypePrivate
	default:
		return 0
	}
}

// IsVideo reports whether stream type is video.
func IsVideo(streamType byte) bool {
	switch streamType {
	case StreamTypeMPEG1Video, StreamTypeMPEG2Video, StreamTypeMPEG4, StreamTypeH264, StreamTypeH265:
		return true
	default:
		return false
	}
}

// IsAudio reports whether stream type is audio.
func IsAudio(streamType byte) bool {
	switch streamType {
	case StreamTypeMPEG1Audio, StreamTypeMPEG2Audio, StreamTypeAAC, StreamTypeG711A, StreamTypeG711U, StreamTypePrivate:
		return true
	default:
		return false
	}
}
