package audio

import (
	"encoding/binary"
)

// RawEncoder is a simple passthrough encoder that wraps PCM in a format
// the Mumble client can at least attempt to process.
// This is NOT real Opus - it's for testing the audio pipeline only.
type RawEncoder struct {
	frameSize int
}

// NewRawEncoder creates a raw encoder for testing.
func NewRawEncoder() *RawEncoder {
	return &RawEncoder{frameSize: FrameSize}
}

// Encode wraps raw PCM in a "fake" Opus-like header for testing.
// This produces invalid Opus but allows the audio pipeline to function.
func (e *RawEncoder) Encode(pcm []byte) ([]byte, error) {
	// For testing: just return the PCM length as a prefix
	// Real implementation would do actual Opus encoding
	// This allows us to test the send/receive pipeline
	return pcm, nil
}

// SetBitrate is a no-op for raw encoder.
func (e *RawEncoder) SetBitrate(bitrate int) error {
	return nil
}

// FrameSize returns the frame size in samples.
func (e *RawEncoder) FrameSize() int {
	return e.frameSize
}

// RawDecoder decodes the "fake" format for testing.
type RawDecoder struct {
	frameSize int
}

// NewRawDecoder creates a raw decoder for testing.
func NewRawDecoder() *RawDecoder {
	return &RawDecoder{frameSize: FrameSize}
}

// Decode extracts raw PCM from our test format.
func (d *RawDecoder) Decode(data []byte) ([]byte, error) {
	return data, nil
}

// FrameSize returns the frame size in samples.
func (d *RawDecoder) FrameSize() int {
	return d.frameSize
}

// TestToneGenerator generates sine wave test tones in PCM format.
type TestToneGenerator struct {
	frequency int
	sampleRate int
	amplitude float64
	phase     float64
}

func NewTestToneGenerator(frequency, sampleRate int, amplitude float64) *TestToneGenerator {
	return &TestToneGenerator{
		frequency:  frequency,
		sampleRate: sampleRate,
		amplitude:  amplitude,
	}
}

// Generate creates a PCM sine wave of the specified duration.
func (g *TestToneGenerator) Generate(durationMs int) []byte {
	frameSize := FrameSize
	frames := (durationMs * g.sampleRate / 1000) / frameSize
	if frames == 0 {
		frames = 1
	}
	samples := frames * frameSize

	pcm := make([]byte, samples*2) // 16-bit mono

	for i := 0; i < samples; i++ {
		sample := int16(g.amplitude * 32767 * sin(2*mathPi*float64(g.frequency)*float64(i)/float64(g.sampleRate)))
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
	}

	return pcm
}

var mathPi = 3.141592653589793

func sin(x float64) float64 {
	// Simple sin approximation for testing
	// Normalize to [-pi, pi]
	for x > mathPi {
		x -= 2 * mathPi
	}
	for x < -mathPi {
		x += 2 * mathPi
	}
	// Taylor series approximation (more terms for better accuracy)
	x2 := x * x
	x3 := x2 * x
	x5 := x2 * x3
	x7 := x2 * x5
	return x - x3/6 + x5/120 - x7/5040
}
