package stream

import (
	"testing"
	"time"
)

func TestJitterBuffer_PushPop(t *testing.T) {
	j := NewJitterBuffer(3, 20*time.Millisecond)

	// Create a test PCM frame (960 samples * 2 bytes = 1920 bytes)
	pcm := make([]byte, 1920)
	for i := range pcm {
		pcm[i] = byte(i % 256)
	}

	deadline := time.Now().Add(60 * time.Millisecond) // 3 frames * 20ms

	// Push a frame
	j.Push(pcm, deadline)

	// Pop should return the frame after deadline
	got, ok := j.Pop(time.Now().Add(70 * time.Millisecond))
	if !ok {
		t.Fatal("expected to pop frame, got false")
	}
	if len(got) != len(pcm) {
		t.Fatalf("expected frame size %d, got %d", len(pcm), len(got))
	}
}

func TestJitterBuffer_Depth(t *testing.T) {
	j := NewJitterBuffer(3, 20*time.Millisecond)

	if got := j.Depth(); got != 0 {
		t.Fatalf("expected depth 0, got %d", got)
	}

	pcm := make([]byte, 1920)
	deadline := time.Now().Add(60 * time.Millisecond)

	j.Push(pcm, deadline)
	if got := j.Depth(); got != 1 {
		t.Fatalf("expected depth 1, got %d", got)
	}
}

func TestJitterBuffer_SilenceOnEmpty(t *testing.T) {
	j := NewJitterBuffer(3, 20*time.Millisecond)

	silence := make([]byte, 1920)
	got, ok := j.Pop(time.Now())
	if ok {
		t.Fatal("expected ok=false on empty buffer, got true")
	}
	if len(got) != len(silence) {
		t.Fatalf("expected silence frame length %d, got %d", len(silence), len(got))
	}
}

func TestJitterBuffer_LateFrameDropped(t *testing.T) {
	j := NewJitterBuffer(3, 20*time.Millisecond)

	pcm := make([]byte, 1920)
	// Frame deadline in the past
	deadline := time.Now().Add(-100 * time.Millisecond)

	j.Push(pcm, deadline)

	// Should return silence because late frame was dropped
	got, ok := j.Pop(time.Now())
	if ok {
		t.Fatal("expected ok=false for late frame, got true")
	}
	if len(got) != 1920 {
		t.Fatalf("expected silence frame, got %d bytes", len(got))
	}
}

func TestJitterBuffer_Close(t *testing.T) {
	j := NewJitterBuffer(3, 20*time.Millisecond)
	j.Close()

	pcm := make([]byte, 1920)
	j.Push(pcm, time.Now().Add(60*time.Millisecond))

	got, ok := j.Pop(time.Now())
	if ok {
		t.Fatal("expected ok=false after close, got true")
	}
	if len(got) != 1920 {
		t.Fatalf("expected silence after close, got %d bytes", len(got))
	}
}
