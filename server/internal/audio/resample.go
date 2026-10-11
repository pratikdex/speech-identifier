// Package audio provides shared PCM audio helpers (resampling, duration
// accounting) used by multiple STT providers.
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

	// Convert byte data to float64 samples
	if len(audioData)%2 != 0 {
		return nil, fmt.Errorf("invalid audio data length: %d (must be even for 16-bit samples)", len(audioData))
	}

	// Convert audio data to samples
	samples := convertAudioToSamples(audioData)

	// Create a custom streamer from the samples
	streamer := newStreamer(samples)

	// Create resampler
	resampler := beep.Resample(3, beep.SampleRate(inputSampleRate), beep.SampleRate(outputSampleRate), streamer)

	bufferSize := 512
	buffer := make([][2]float64, bufferSize)

	var finalSamples [][2]float64

	if outputSampleRate <= inputSampleRate {
		// For downsampling/same rate: overwrite original samples in-place to save memory
		writePos := 0

		for {
			n, ok := resampler.Stream(buffer)
			if n > 0 {
				copy(samples[writePos:], buffer[:n])
				writePos += n
			}
			if !ok {
				break
			}
		}
		finalSamples = samples[:writePos]
	} else {
		// For upsampling: use separate slice as we need more space
		numSamples := len(audioData) / 2
		expectedSamples := int(float64(numSamples) * float64(outputSampleRate) / float64(inputSampleRate))
		resampledSamples := make([][2]float64, 0, expectedSamples)

		for {
			n, ok := resampler.Stream(buffer)
			if n > 0 {
				resampledSamples = append(resampledSamples, buffer[:n]...)
			}
			if !ok {
				break
			}
		}
		finalSamples = resampledSamples
	}

	// Convert back to bytes (16-bit signed little-endian, mono)
	outputData := make([]byte, len(finalSamples)*2)
	for i, sample := range finalSamples {
		// Use left channel only for mono output
		floatSample := sample[0]
		if floatSample > 1.0 {
			floatSample = 1.0
		}
		if floatSample < -1.0 {
			floatSample = -1.0
		}
		intSample := int16(floatSample * 32767)
		binary.LittleEndian.PutUint16(outputData[i*2:(i+1)*2], uint16(intSample))
	}

	return outputData, nil
}

// pcmStreamer implements beep.Streamer for raw PCM data
type pcmStreamer struct {
	samples [][2]float64
	pos     int
}

// convertAudioToSamples converts raw audio data to float64 samples
func convertAudioToSamples(audioData []byte) [][2]float64 {
	numSamples := len(audioData) / 2
	samples := make([][2]float64, numSamples)

	// Convert 16-bit signed little-endian samples to float64 (mono, so copy to both channels)
	for i := range numSamples {
		sample := int16(binary.LittleEndian.Uint16(audioData[i*2 : (i+1)*2]))
		floatSample := float64(sample) / 32768.0 // Normalize to [-1, 1]
		samples[i][0] = floatSample              // Left channel
		samples[i][1] = floatSample              // Right channel (same as left for mono)
	}

	return samples
}

// newStreamer creates a new pcmStreamer from pre-converted samples
func newStreamer(samples [][2]float64) *pcmStreamer {
	return &pcmStreamer{
		samples: samples,
		pos:     0,
	}
}

func (s *pcmStreamer) Stream(buf [][2]float64) (n int, ok bool) {
	if s.pos >= len(s.samples) {
		return 0, false
	}

	n = len(buf)
	if s.pos+n > len(s.samples) {
		n = len(s.samples) - s.pos
	}

	copy(buf[:n], s.samples[s.pos:s.pos+n])
	s.pos += n

	return n, true
}

func (s *pcmStreamer) Err() error {
	return nil
}
