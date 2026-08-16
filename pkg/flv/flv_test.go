package flv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimeToRTP(t *testing.T) {
	// Reolink camera has 20 FPS
	// Video timestamp increases by 50ms, SampleRate 90000, RTP timestamp increases by 4500
	// Audio timestamp increases by 64ms, SampleRate 16000, RTP timestamp increases by 1024
	frameN := 1
	for i := 0; i < 32; i++ {
		// 1000ms/(90000/4500) = 50ms
		require.Equal(t, uint32(frameN*4500), TimeToRTP(uint32(frameN*50), 90000))
		// 1000ms/(16000/1024) = 64ms
		require.Equal(t, uint32(frameN*1024), TimeToRTP(uint32(frameN*64), 16000))
		frameN *= 2
	}
}

func TestReadSI24(t *testing.T) {
	require.Equal(t, int32(0), readSI24([]byte{0, 0, 0}))
	require.Equal(t, int32(33), readSI24([]byte{0, 0, 33}))
	require.Equal(t, int32(84), readSI24([]byte{0, 0, 84}))
	// negative (sign bit in high byte)
	require.Equal(t, int32(-1), readSI24([]byte{0xff, 0xff, 0xff}))
}

func TestCompositionToRTP(t *testing.T) {
	// monibuca: CTS in ms; FrameFlow MSE uses track timescale (=90000)
	require.Equal(t, uint16(0), compositionToRTP(0, 90000))
	require.Equal(t, uint16(2970), compositionToRTP(33, 90000)) // 33ms * 90
	require.Equal(t, uint16(7560), compositionToRTP(84, 90000))
	require.Equal(t, uint16(0), compositionToRTP(-10, 90000))
}
