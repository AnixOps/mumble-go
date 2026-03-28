//go:build opus
// +build opus

package client

import (
	"testing"
)

func TestAudioSetRawCodec(t *testing.T) {
	cfg := DefaultAudioConfig()
	a := NewAudio(cfg)

	// Initially no encoder set
	if a == nil {
		t.Fatal("NewAudio returned nil")
	}

	// Set raw codec
	err := a.SetRawCodec()
	if err != nil {
		t.Fatalf("SetRawCodec failed: %v", err)
	}
}

func TestAudioConfig(t *testing.T) {
	cfg := DefaultAudioConfig()
	if cfg.Bandwidth != 50000 {
		t.Errorf("Default bandwidth = %d, want 50000", cfg.Bandwidth)
	}
	if !cfg.EnableReceive {
		t.Error("EnableReceive should be true by default")
	}
}

func TestNewAudio(t *testing.T) {
	cfg := DefaultAudioConfig()
	a := NewAudio(cfg)
	if a == nil {
		t.Fatal("NewAudio returned nil")
	}

	if a.IsEnabled() != true {
		t.Error("Audio should be enabled by default")
	}

	// Test SetEnabled
	a.SetEnabled(false)
	if a.IsEnabled() {
		t.Error("Audio should be disabled after SetEnabled(false)")
	}

	a.SetEnabled(true)
	if !a.IsEnabled() {
		t.Error("Audio should be enabled after SetEnabled(true)")
	}
}

func TestAudioBandwidth(t *testing.T) {
	cfg := DefaultAudioConfig()
	a := NewAudio(cfg)

	a.SetBandwidth(40000)
	// SetBandwidth should not error
}

func TestAudioSetOpusCodec(t *testing.T) {
	cfg := DefaultAudioConfig()
	a := NewAudio(cfg)

	// Set real Opus codec
	err := a.SetOpusCodec()
	if err != nil {
		t.Fatalf("SetOpusCodec failed: %v", err)
	}
}
