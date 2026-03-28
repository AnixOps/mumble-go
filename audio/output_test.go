package audio

import (
	"bytes"
	"testing"
	"time"
)

func TestOutputBasic(t *testing.T) {
	out := NewOutput()
	if out == nil {
		t.Fatal("NewOutput returned nil")
	}

	// Test SetBandwidth
	out.SetBandwidth(40000)

	// Test SetTarget
	out.SetTarget(TargetWhisper)
	out.SetTarget(TargetNormal)

	// Test BufferDuration with empty buffer
	dur := out.BufferDuration()
	if dur != 0 {
		t.Errorf("Empty buffer duration = %v, want 0", dur)
	}

	// Test ClearBuffer
	out.ClearBuffer()
}

func TestOutputAddPCM(t *testing.T) {
	out := NewOutput()

	// Add some PCM data (960 samples = 20ms at 48kHz)
	pcm := make([]byte, 1920) // 960 * 2 bytes
	for i := range pcm {
		pcm[i] = byte(i)
	}

	out.AddPCM(pcm)

	// Check buffer duration (should be 20ms)
	dur := out.BufferDuration()
	expected := 20 * time.Millisecond
	if dur != expected {
		t.Errorf("BufferDuration = %v, want %v", dur, expected)
	}

	// Clear buffer
	out.ClearBuffer()
	dur = out.BufferDuration()
	if dur != 0 {
		t.Errorf("After clear, BufferDuration = %v, want 0", dur)
	}
}

func TestOutputAddPCMSplit(t *testing.T) {
	out := NewOutput()

	// Add PCM smaller than one frame
	pcm := make([]byte, 500) // Less than 960 samples
	for i := range pcm {
		pcm[i] = byte(i)
	}

	out.AddPCM(pcm)

	// Should still buffer it
	dur := out.BufferDuration()
	if dur == 0 {
		t.Error("BufferDuration should not be 0 after adding PCM")
	}
}

func TestOutputNoEncoderNoWriter(t *testing.T) {
	out := NewOutput()

	// Add PCM
	pcm := make([]byte, 1920)
	out.AddPCM(pcm)

	// Send without encoder or writer - should return 0, nil
	frames, err := out.Send()
	if err != nil {
		t.Errorf("Send without encoder/writer returned error: %v", err)
	}
	if frames != 0 {
		t.Errorf("Send without encoder/writer returned frames=%d, want 0", frames)
	}
}

type mockWriter struct {
	buf bytes.Buffer
}

func (m *mockWriter) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func TestOutputSendWithMockWriter(t *testing.T) {
	out := NewOutput()

	writer := &mockWriter{}
	out.SetWriter(writer)

	// Set raw encoder (testing mode)
	rawEnc := NewRawEncoder()
	enc, err := NewOpusEncoder(rawEnc)
	if err != nil {
		t.Fatalf("NewOpusEncoder failed: %v", err)
	}
	out.SetEncoder(enc)

	// Add PCM
	pcm := make([]byte, 1920)
	for i := range pcm {
		pcm[i] = byte(i % 256)
	}
	out.AddPCM(pcm)

	// Send
	frames, err := out.Send()
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
	if frames == 0 {
		t.Error("Send returned 0 frames")
	}

	// Check that data was written
	if writer.buf.Len() == 0 {
		t.Error("No data written to writer")
	}

	// Check buffer is cleared after send
	dur := out.BufferDuration()
	if dur != 0 {
		t.Errorf("BufferDuration after send = %v, want 0", dur)
	}
}

func TestOutputSequence(t *testing.T) {
	out := NewOutput()

	writer := &mockWriter{}
	out.SetWriter(writer)

	rawEnc := NewRawEncoder()
	enc, _ := NewOpusEncoder(rawEnc)
	out.SetEncoder(enc)
	out.SetWriter(writer)

	// Send first batch
	pcm := make([]byte, 1920)
	out.AddPCM(pcm)
	frames1, _ := out.Send()

	// Small delay
	time.Sleep(10 * time.Millisecond)

	// Send second batch
	out.AddPCM(pcm)
	frames2, _ := out.Send()

	if frames1 != frames2 {
		t.Errorf("Frames sent differ: first=%d second=%d", frames1, frames2)
	}
}

func TestEncodeVarint(t *testing.T) {
	tests := []struct {
		n        uint64
		expected []byte
	}{
		{0, []byte{0}},
		{1, []byte{1}},
		{127, []byte{127}},
		{128, []byte{0x80, 0x01}},
		{300, []byte{0xAC, 0x02}},
		{1000, []byte{0xE8, 0x07}},
	}

	for _, tt := range tests {
		got := encodeVarint(tt.n)
		if !bytes.Equal(got, tt.expected) {
			t.Errorf("encodeVarint(%d) = %x, want %x", tt.n, got, tt.expected)
		}
	}
}
