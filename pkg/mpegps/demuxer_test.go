package mpegps

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDemuxerPES(t *testing.T) {
	// Minimal synthetic PS: pack header + PES with H264-looking payload
	var buf bytes.Buffer

	// pack header 00 00 01 BA + 10 bytes (SCR/mux/stuffing=0)
	pack := []byte{
		0x00, 0x00, 0x01, 0xBA,
		0x44, 0x00, 0x04, 0x00, 0x04, 0x01, // SCR-ish
		0x01, 0x89, 0xC3, // mux rate
		0xF8, // stuffing len 0
	}
	buf.Write(pack)

	payload := []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42} // fake annexb SPS start
	pesHeader := []byte{
		0x00, 0x00, 0x01, 0xE0, // video stream
		0x00, 0x00, // length filled below
		0x80, 0x00, 0x00, // no PTS, header data len 0
	}
	pesLen := len(pesHeader) - 6 + len(payload)
	binary.BigEndian.PutUint16(pesHeader[4:6], uint16(pesLen))
	buf.Write(pesHeader)
	buf.Write(payload)

	d := NewDemuxer()
	_, _ = d.Write(buf.Bytes())
	pkt, err := d.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if pkt.StreamID != 0xE0 {
		t.Fatalf("stream id = %x", pkt.StreamID)
	}
	if pkt.StreamType != StreamTypeH264 {
		t.Fatalf("stream type = %x", pkt.StreamType)
	}
	if !bytes.Equal(pkt.Payload, payload) {
		t.Fatalf("payload mismatch: %x", pkt.Payload)
	}
}

func TestDemuxerPSM(t *testing.T) {
	var buf bytes.Buffer
	// pack
	buf.Write([]byte{0x00, 0x00, 0x01, 0xBA, 0x44, 0x00, 0x04, 0x00, 0x04, 0x01, 0x01, 0x89, 0xC3, 0xF8})

	// PSM: map stream E0 -> H265 (0x24)
	psm := []byte{
		0x00, 0x00, 0x01, 0xBC,
		0x00, 0x0E, // length
		0xE1, 0xFF, // current_next | version | marker, reserved
		0x00, 0x00, // program_stream_info_length
		0x00, 0x04, // elementary_stream_map_length
		0x24, 0xE0, 0x00, 0x00, // H265, esid E0, info len 0
		0x00, 0x00, // CRC placeholder (ignored by our parser)
	}
	// fix length: bytes after length field
	binary.BigEndian.PutUint16(psm[4:6], uint16(len(psm)-6))
	buf.Write(psm)

	payload := []byte{0x00, 0x00, 0x01, 0x40}
	pes := []byte{0x00, 0x00, 0x01, 0xE0, 0x00, 0x00, 0x80, 0x00, 0x00}
	binary.BigEndian.PutUint16(pes[4:6], uint16(3+len(payload)))
	buf.Write(pes)
	buf.Write(payload)

	d := NewDemuxer()
	_, _ = d.Write(buf.Bytes())
	pkt, err := d.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if pkt.StreamType != StreamTypeH265 {
		t.Fatalf("want H265 got %x", pkt.StreamType)
	}
}
