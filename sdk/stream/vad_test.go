package stream

import (
	"math"
	"testing"
)

func TestVAD_Process(t *testing.T) {
	v := NewVAD(500, func(speaking bool) {
		_ = speaking // callback validates state changes
	})

	// Silence - should not trigger speech
	silence := make([]byte, 1920) // 960 samples * 2 bytes
	isSpeaking := v.Process(silence)
	if isSpeaking {
		t.Fatal("expected silence to not trigger speech")
	}

	// Loud signal - create actual PCM 16-bit max values
	loud := make([]byte, 1920)
	for i := 0; i < 960; i++ {
		// PCM 16-bit little endian, max amplitude = 32767
		loud[i*2] = 0xFF
		loud[i*2+1] = 0x7F
	}
	isSpeaking = v.Process(loud)
	if !isSpeaking {
		t.Fatal("expected loud signal to trigger speech")
	}
}

func TestVAD_IsSpeaking(t *testing.T) {
	v := NewVAD(500, nil)

	silence := make([]byte, 1920)
	v.Process(silence)
	if v.IsSpeaking() {
		t.Fatal("expected IsSpeaking=false after silence")
	}

	// Loud signal
	loud := make([]byte, 1920)
	for i := 0; i < 960; i++ {
		loud[i*2] = 0xFF
		loud[i*2+1] = 0x7F
	}
	v.Process(loud)
	if !v.IsSpeaking() {
		t.Fatal("expected IsSpeaking=true after loud signal")
	}
}

func TestVAD_Threshold(t *testing.T) {
	v := NewVAD(1000, nil)

	// Signal just below threshold - low amplitude
	below := make([]byte, 1920)
	for i := 0; i < 960; i++ {
		// Very low amplitude (10)
		below[i*2] = 10
		below[i*2+1] = 0
	}
	if v.Process(below) {
		t.Fatal("expected below threshold to not trigger speech")
	}

	// Signal above threshold - higher amplitude
	at := make([]byte, 1920)
	for i := 0; i < 960; i++ {
		// Higher amplitude (1000)
		at[i*2] = 0xE8
		at[i*2+1] = 0x03
	}
	v2 := NewVAD(100, nil)
	if !v2.Process(at) {
		t.Fatal("expected above threshold to trigger speech")
	}
}

func TestVAD_Callback(t *testing.T) {
	calls := 0
	v := NewVAD(500, func(speaking bool) {
		calls++
	})

	// First process - should trigger callback on state change to speaking
	loud := make([]byte, 1920)
	for i := 0; i < 960; i++ {
		loud[i*2] = 0xFF
		loud[i*2+1] = 0x7F
	}
	v.Process(loud)
	v.Process(loud) // Same state, no callback

	if calls != 1 {
		t.Fatalf("expected 1 callback (on state change), got %d", calls)
	}
}

func TestComputeRMS(t *testing.T) {
	tests := []struct {
		name  string
		pcm   []byte
		min   float64
		max   float64
	}{
		{
			name:  "silence",
			pcm:   make([]byte, 1920),
			min:   0,
			max:   0.1,
		},
		{
			name: "max signal",
			pcm: func() []byte {
				b := make([]byte, 1920)
				for i := 0; i < 960; i++ {
					// PCM 16-bit max amplitude 32767
					b[i*2] = 0xFF
					b[i*2+1] = 0x7F
				}
				return b
			}(),
			min: 32000,
			max: 34000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rms := computeRMS(tt.pcm)
			if rms < tt.min || rms > tt.max {
				t.Errorf("computeRMS() = %v, want between %v and %v", rms, tt.min, tt.max)
			}
		})
	}
}

func TestComputeRMS_Empty(t *testing.T) {
	rms := computeRMS([]byte{})
	if !math.IsNaN(rms) && rms != 0 {
		t.Fatalf("computeRMS(empty) should return 0, got %v", rms)
	}
}
