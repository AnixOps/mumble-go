package client

import (
	"crypto/tls"
	"testing"
)

func TestClientState(t *testing.T) {
	cfg := Config{
		Address:  "localhost:64738",
		Username: "test",
		TLS:      &tls.Config{InsecureSkipVerify: true},
		Audio:    DefaultAudioConfig(),
	}

	c := New(cfg)
	if c == nil {
		t.Fatal("New returned nil")
	}

	if c.State() == nil {
		t.Error("State() should not return nil")
	}

	if c.Connected() {
		t.Error("New client should not be connected")
	}

	if c.SupportsOpus() {
		t.Error("New client should not support Opus before connection")
	}
}

func TestClientAudio(t *testing.T) {
	cfg := Config{
		Address:  "localhost:64738",
		Username: "test",
		TLS:      &tls.Config{InsecureSkipVerify: true},
		Audio:    DefaultAudioConfig(),
	}

	c := New(cfg)
	if c == nil {
		t.Fatal("New returned nil")
	}

	if c.Audio() == nil {
		t.Error("Audio() should not return nil")
	}
}
