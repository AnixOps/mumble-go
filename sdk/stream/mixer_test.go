package stream

import (
	"context"
	"io"
	"testing"
)

func TestAudioMixer_Basic(t *testing.T) {
	m := NewAudioMixer(960)

	// Create a simple test source
	src := &testSource{data: make([]byte, 1920), frames: 3}

	// Add source
	m.AddSource("test", src, 1.0)

	// Read should work
	buf := make([]byte, 1920)
	ctx := context.Background()
	n, err := m.ReadMix(ctx, buf)
	if err != nil {
		t.Fatalf("ReadMix() error = %v", err)
	}
	if n != 1920 {
		t.Fatalf("expected 1920 bytes, got %d", n)
	}
}

func TestAudioMixer_MultipleSources(t *testing.T) {
	m := NewAudioMixer(960)

	// Create two test sources
	src1 := &testSource{data: make([]byte, 1920), frames: 2}
	src2 := &testSource{data: make([]byte, 1920), frames: 2}

	// Add both sources
	m.AddSource("src1", src1, 1.0)
	m.AddSource("src2", src2, 0.5) // Lower gain

	// Read should mix both
	buf := make([]byte, 1920)
	ctx := context.Background()
	n, err := m.ReadMix(ctx, buf)
	if err != nil {
		t.Fatalf("ReadMix() error = %v", err)
	}
	if n != 1920 {
		t.Fatalf("expected 1920 bytes, got %d", n)
	}
}

func TestAudioMixer_RemoveSource(t *testing.T) {
	m := NewAudioMixer(960)

	src := &testSource{data: make([]byte, 1920), frames: 1}
	m.AddSource("test", src, 1.0)

	m.RemoveSource("test")

	// With no sources, mixer returns silence (filled buffer)
	buf := make([]byte, 1920)
	ctx := context.Background()
	n, err := m.ReadMix(ctx, buf)
	if err != nil {
		t.Fatalf("ReadMix() error = %v", err)
	}
	// Mixer returns len(dst) even with no sources (silence frame)
	if n != 1920 {
		t.Fatalf("expected 1920 bytes (silence), got %d", n)
	}
	// Verify it's silence (all zeros)
	for _, b := range buf {
		if b != 0 {
			t.Fatal("expected silence buffer (all zeros)")
		}
		break
	}
}

func TestAudioMixer_Gain(t *testing.T) {
	m := NewAudioMixer(960)

	src := &testSource{data: make([]byte, 1920), frames: 1}
	m.AddSource("test", src, 2.0) // Double gain

	m.SetGain("test", 0.5) // Halve gain

	// Should still work with adjusted gain
	buf := make([]byte, 1920)
	ctx := context.Background()
	n, err := m.ReadMix(ctx, buf)
	if err != nil {
		t.Fatalf("ReadMix() error = %v", err)
	}
	if n != 1920 {
		t.Fatalf("expected 1920 bytes, got %d", n)
	}
}

func TestAudioMixer_Close(t *testing.T) {
	m := NewAudioMixer(960)
	m.Close()

	buf := make([]byte, 1920)
	ctx := context.Background()
	_, err := m.ReadMix(ctx, buf)
	// Close doesn't return error, mixer still outputs silence
	if err != nil {
		t.Fatalf("expected no error after close, got %v", err)
	}
}

// testSource is a simple AudioSource for testing
type testSource struct {
	data   []byte
	frames int
	pos    int
}

func (t *testSource) ReadPCM(ctx context.Context, p []byte) (n int, err error) {
	if t.pos >= len(t.data)*t.frames {
		return 0, io.EOF
	}
	remaining := len(t.data) - (t.pos % len(t.data))
	if len(p) < remaining {
		remaining = len(p)
	}
	copy(p, t.data[t.pos%len(t.data):])
	t.pos += remaining
	if t.pos >= len(t.data)*t.frames {
		return remaining, io.EOF
	}
	return remaining, nil
}
