package audio

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tone returns one second of a 440 Hz sine as 16-bit PCM at rate.
func tone(rate int) []byte {
	pcm := make([]byte, 0, rate*2)
	for i := range rate {
		v := int16(0.5 * 32767 * math.Sin(2*math.Pi*440*float64(i)/float64(rate)))
		pcm = binary.LittleEndian.AppendUint16(pcm, uint16(v))
	}
	return pcm
}

// crossings counts sign changes; a 440 Hz tone has ~880 per second at any rate.
func crossings(pcm []byte) int {
	n, prev := 0, int16(0)
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int16(binary.LittleEndian.Uint16(pcm[i:]))
		if (v < 0) != (prev < 0) {
			n++
		}
		prev = v
	}
	return n
}

func TestResample_KeepsDurationAndPitch(t *testing.T) {
	for _, in := range []int{8000, 44100, 48000} {
		out, err := Resample(tone(in), in, TargetRate)
		require.NoError(t, err)
		assert.InDelta(t, TargetRate*2, len(out), 64, "rate %d: length", in)
		assert.InDelta(t, 880, crossings(out), 4, "rate %d: pitch", in)
	}
}

func TestResample_RejectsBadInput(t *testing.T) {
	_, err := Resample(nil, 48000, TargetRate)
	assert.Error(t, err)
	_, err = Resample([]byte{1, 2, 3}, 48000, TargetRate)
	assert.Error(t, err)
}
