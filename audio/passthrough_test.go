package audio

import (
	"testing"
)

func TestRawEncoder(t *testing.T) {
	enc := NewRawEncoder()
	if enc == nil {
		t.Fatal("NewRawEncoder returned nil")
	}

	// Test encoding
	pcm := make([]byte, 1920) // 20ms of 48kHz mono
	for i := range pcm {
		pcm[i] = byte(i)
	}

	encoded, err := enc.Encode(pcm)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(encoded) != len(pcm) {
		t.Errorf("Encode length mismatch: got %d, want %d", len(encoded), len(pcm))
	}

	// Test SetBitrate (should be no-op)
	if err := enc.SetBitrate(50000); err != nil {
		t.Errorf("SetBitrate failed: %v", err)
	}

	// Test FrameSize
	if enc.FrameSize() != FrameSize {
		t.Errorf("FrameSize mismatch: got %d, want %d", enc.FrameSize(), FrameSize)
	}
}

func TestRawDecoder(t *testing.T) {
	dec := NewRawDecoder()
	if dec == nil {
		t.Fatal("NewRawDecoder returned nil")
	}

	// Test decoding
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i * 2)
	}

	decoded, err := dec.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(decoded) != len(data) {
		t.Errorf("Decode length mismatch: got %d, want %d", len(decoded), len(data))
	}

	// Test FrameSize
	if dec.FrameSize() != FrameSize {
		t.Errorf("FrameSize mismatch: got %d, want %d", dec.FrameSize(), FrameSize)
	}
}

func TestTestToneGenerator(t *testing.T) {
	gen := NewTestToneGenerator(440, 48000, 0.5)
	if gen == nil {
		t.Fatal("NewTestToneGenerator returned nil")
	}

	// Generate 100ms of tone
	pcm := gen.Generate(100)
	expectedSamples := (100 * 48000 / 1000) / FrameSize * FrameSize
	expectedBytes := expectedSamples * 2

	if len(pcm) != expectedBytes {
		t.Errorf("Generate length mismatch: got %d bytes, want %d bytes", len(pcm), expectedBytes)
	}

	// Check that we have non-zero samples (not just silence)
	hasNonZero := false
	for i := 0; i < len(pcm); i += 100 {
		if pcm[i] != 0 || pcm[i+1] != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("Generated PCM is all zeros")
	}
}

func TestSin(t *testing.T) {
	// Test sin function - use points where approximation works well
	tests := []struct {
		input    float64
		expected float64
		approx   bool
	}{
		{0, 0, true},            // sin(0) = 0
		{mathPi / 6, 0.5, true}, // sin(pi/6) = 0.5
		{mathPi / 4, 0.707, true}, // sin(pi/4) ≈ 0.707
		{mathPi / 3, 0.866, true}, // sin(pi/3) ≈ 0.866
		{mathPi / 2, 1, true},    // sin(pi/2) ≈ 1
	}

	for _, tt := range tests {
		got := sin(tt.input)
		if tt.approx {
			// Allow 2% error for Taylor approximation
			ratio := got / tt.expected
			if ratio < 0.98 || ratio > 1.02 {
				t.Errorf("sin(%v) = %v, want approximately %v", tt.input, got, tt.expected)
			}
		}
	}
}
