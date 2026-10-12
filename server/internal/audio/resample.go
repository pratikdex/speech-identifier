// Package audio provides the PCM resampler shared by the STT providers.
package audio

import (
	"encoding/binary"
	"fmt"

	"github.com/gopxl/beep/v2"
)

// TargetRate is the sample rate (Hz) that streaming STT providers expect
// their audio to be resampled to before it is sent over the wire.
const TargetRate = 16000

// Resample resamples 16-bit signed little-endian mono PCM audio data from
// inputSampleRate to outputSampleRate using the beep library in-memory.
func Resample(audioData []byte, inputSampleRate int, outputSampleRate int) ([]byte, error) {
	if len(audioData) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}
	if len(audioData)%2 != 0 {
		return nil, fmt.Errorf("invalid audio data length: %d (must be even for 16-bit samples)", len(audioData))
	}

	in := &pcmStreamer{samples: make([][2]float64, len(audioData)/2)}
	for i := range in.samples {
		v := float64(int16(binary.LittleEndian.Uint16(audioData[i*2:]))) / 32768.0
		in.samples[i] = [2]float64{v, v} // beep streams stereo; mono goes in both channels
	}

	resampler := beep.Resample(3, beep.SampleRate(inputSampleRate), beep.SampleRate(outputSampleRate), in)
	out := make([]byte, 0, len(audioData)*outputSampleRate/inputSampleRate+2)
	buf := make([][2]float64, 512)
	for {
		n, ok := resampler.Stream(buf)
		for _, s := range buf[:n] {
			out = binary.LittleEndian.AppendUint16(out, uint16(int16(max(-1, min(1, s[0]))*32767)))
		}
		if !ok {
			return out, nil
		}
	}
}

// pcmStreamer implements beep.Streamer over in-memory samples.
type pcmStreamer struct {
	samples [][2]float64
	pos     int
}

func (s *pcmStreamer) Stream(buf [][2]float64) (n int, ok bool) {
	if s.pos >= len(s.samples) {
		return 0, false
	}
	n = copy(buf, s.samples[s.pos:])
	s.pos += n
	return n, true
}

func (s *pcmStreamer) Err() error { return nil }
