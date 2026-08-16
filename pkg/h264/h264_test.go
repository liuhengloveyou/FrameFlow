package h264

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeConfig(t *testing.T) {
	s := "01640033ffe1000c67640033ac1514a02800f19001000468ee3cb0"
	src, err := hex.DecodeString(s)
	require.Nil(t, err)

	profile, sps, pps := DecodeConfig(src)
	require.NotNil(t, profile)
	require.NotNil(t, sps)
	require.NotNil(t, pps)

	dst := EncodeConfig(sps, pps)
	require.Equal(t, src, dst)
}

func TestDecodeSPS(t *testing.T) {
	s := "Z0IAMukAUAHjQgAAB9IAAOqcCAA=" // Amcrest AD410
	b, err := base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps := DecodeSPS(b)
	require.Equal(t, uint16(2560), sps.Width())
	require.Equal(t, uint16(1920), sps.Height())

	s = "R00AKZmgHgCJ+WEAAAMD6AAATiCE" // Sonoff
	b, err = base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps = DecodeSPS(b)
	require.Equal(t, uint16(1920), sps.Width())
	require.Equal(t, uint16(1080), sps.Height())

	s = "Z01AMqaAKAC1kAA=" // Dahua
	b, err = base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps = DecodeSPS(b)
	require.Equal(t, uint16(2560), sps.Width())
	require.Equal(t, uint16(1440), sps.Height())

	s = "Z2QAM6wVFKAoAPGQ" // Reolink
	b, err = base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps = DecodeSPS(b)
	require.Equal(t, uint16(2560), sps.Width())
	require.Equal(t, uint16(1920), sps.Height())

	s = "Z2QAKKwa0AoAt03AQEBQAAADABAAAAMB6PFCKg==" // TP-Link
	b, err = base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps = DecodeSPS(b)
	require.Equal(t, uint16(1280), sps.Width())
	require.Equal(t, uint16(720), sps.Height())

	s = "Z2QAFqwa0BQF/yzcBAQFAAADAAEAAAMAHo8UIqA=" // TP-Link sub
	b, err = base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps = DecodeSPS(b)
	require.Equal(t, uint16(640), sps.Width())
	require.Equal(t, uint16(360), sps.Height())
}

func TestGetProfileLevelID(t *testing.T) {
	// OpenIPC https://github.com/OpenIPC
	s := "profile-level-id=0033e7; packetization-mode=1; "
	profile := GetProfileLevelID(s)
	require.Equal(t, "640029", profile)

	// Eufy T8400 https://github.com/liuhengloveyou/FrameFlow/issues/155
	s = "packetization-mode=1;profile-level-id=276400"
	profile = GetProfileLevelID(s)
	require.Equal(t, "640029", profile)
}

func TestDecodeSPS2(t *testing.T) {
	s := "6764001fad84010c20086100430802184010c200843b50740932"
	b, err := hex.DecodeString(s)
	require.Nil(t, err)

	sps := DecodeSPS(b)
	require.Equal(t, uint16(928), sps.Width())
	require.Equal(t, uint16(576), sps.Height())

	s = "Z2QAHq2EAQwgCGEAQwgCGEAQwgCEO1BQF/yzcBAQFAAAD6AAAXcCEA==" // unknown
	b, err = base64.StdEncoding.DecodeString(s)
	require.Nil(t, err)

	sps = DecodeSPS(b)
	require.Equal(t, uint16(640), sps.Width())
	require.Equal(t, uint16(360), sps.Height())
}

func TestEnsureKeyframeParamSets(t *testing.T) {
	sps := []byte{0x67, 0x42, 0x00, 0x1e}
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := JoinNALU([]byte{0x65, 0x88, 0x84, 0x00})

	got := EnsureKeyframeParamSets(idr, sps, pps)
	if !HasNALUType(got, NALUTypeSPS) || !HasNALUType(got, NALUTypePPS) || !IsKeyframe(got) {
		t.Fatalf("expected SPS+PPS+IDR, types=%v", NALUTypes(got))
	}

	// already has param sets — unchanged length
	full := Join(JoinNALU(sps, pps), idr)
	got2 := EnsureKeyframeParamSets(full, sps, pps)
	if len(got2) != len(full) {
		t.Fatalf("should not double-inject: %d vs %d", len(got2), len(full))
	}

	p := JoinNALU([]byte{0x41, 0x00}) // P-frame
	got3 := EnsureKeyframeParamSets(p, sps, pps)
	if len(got3) != len(p) {
		t.Fatal("P-frame must not get param sets")
	}
}

func TestParamSetsOnly(t *testing.T) {
	sps := JoinNALU([]byte{0x67, 0x42, 0x00, 0x1e})
	if !ParamSetsOnly(sps) {
		t.Fatal("SPS-only should be param sets")
	}
	idr := JoinNALU([]byte{0x65, 0x88})
	if ParamSetsOnly(idr) {
		t.Fatal("IDR is not param-only")
	}
}

func TestAVCCToCodec(t *testing.T) {
	s := "000000196764001fac2484014016ec0440000003004000000c23c60c920000000568ee32c8b0000000d365"
	b, _ := hex.DecodeString(s)
	codec := AVCCToCodec(b)
	require.Equal(t, "packetization-mode=1;profile-level-id=64001f;sprop-parameter-sets=Z2QAH6wkhAFAFuwEQAAAAwBAAAAMI8YMkg==,aO4yyLA=", codec.FmtpLine)
}
