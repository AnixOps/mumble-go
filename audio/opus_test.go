//go:build opus
// +build opus

package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpusEncodeDecode(t *testing.T) {
	enc, err := NewOpusEnc(SampleRate, Channels, FrameSize)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	dec, err := NewOpusDec(SampleRate, Channels, FrameSize)
	if err != nil {
		t.Skipf("Opus decoder not available: %v", err)
	}

	// Generate test PCM (440Hz sine wave, 20ms)
	pcm := generateTestPCM(440, 20, SampleRate, 0.3)

	// Encode
	opusData, err := enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	fmt.Printf("Encoded %d bytes of PCM to %d bytes of opus\n", len(pcm), len(opusData))

	// Decode
	decodedPCM, err := dec.Decode(opusData)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	fmt.Printf("Decoded to %d bytes of PCM\n", len(decodedPCM))

	// Verify length matches
	if len(decodedPCM) != len(pcm) {
		t.Errorf("Decoded PCM length %d != original %d", len(decodedPCM), len(pcm))
	}
}

func TestOpusRoundTrip(t *testing.T) {
	enc, err := NewOpusEnc(SampleRate, Channels, FrameSize)
	if err != nil {
		t.Skipf("Opus not available: %v", err)
	}

	dec, err := NewOpusDec(SampleRate, Channels, FrameSize)
	if err != nil {
		t.Skipf("Opus decoder not available: %v", err)
	}

	// Test with 500ms of audio
	durationMs := 500
	pcm := generateTestPCM(440, durationMs, SampleRate, 0.3)

	// Encode all frames
	opusFrames := make([][]byte, 0)
	frameBytes := FrameSize * 2
	for len(pcm) >= frameBytes {
		frame := pcm[:frameBytes]
		pcm = pcm[frameBytes:]
		encoded, err := enc.Encode(frame)
		if err != nil {
			t.Fatalf("Encode frame failed: %v", err)
		}
		opusFrames = append(opusFrames, encoded)
	}
	fmt.Printf("Encoded %d frames\n", len(opusFrames))

	// Decode all frames
	decodedPCM := make([]byte, 0)
	for _, opusFrame := range opusFrames {
		decoded, err := dec.Decode(opusFrame)
		if err != nil {
			t.Fatalf("Decode frame failed: %v", err)
		}
		decodedPCM = append(decodedPCM, decoded...)
	}
	fmt.Printf("Decoded to %d bytes (%d ms)\n", len(decodedPCM), len(decodedPCM)/2/(SampleRate/1000))

	// Check approximate length
	expectedSamples := durationMs * SampleRate / 1000
	if len(decodedPCM)/2 != expectedSamples {
		t.Errorf("Decoded samples %d != expected %d", len(decodedPCM)/2, expectedSamples)
	}
}

func generateTestPCM(frequency, durationMs, sampleRate int, amplitude float64) []byte {
	samples := durationMs * sampleRate / 1000
	pcm := make([]byte, samples*2)

	for i := 0; i < samples; i++ {
		v := amplitude * math.Sin(2*math.Pi*float64(frequency)*float64(i)/float64(sampleRate))
		sample := int16(v * 32767)
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
	}
	return pcm
}
