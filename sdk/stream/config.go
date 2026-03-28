package stream

import (
	"time"
)

// DefaultStreamConfig returns a Config with sensible defaults.
func DefaultStreamConfig() *StreamConfig {
	return &StreamConfig{
		// Opus encoding
		Bitrate:    64000,
		Complexity: 10,

		// Jitter buffer: 3 frames = 60ms at 20ms/frame
		BufferDepth: 3,

		// VAD: energy-based, enabled by default for bandwidth saving
		VADEnabled:      true,
		EnergyThreshold: 500,

		// Reconnection: enabled by default
		Reconnect:            true,
		MaxAttempts:          10,
		InitialDelay:         1 * time.Second,
		MaxDelay:             30 * time.Second,
		BackoffMultiplier:    2.0,
		ReconnectBufferSize:  100, // frames to buffer during reconnect (~2s)
	}
}

// StreamConfig controls StreamSender behavior.
type StreamConfig struct {
	// Opus encoding
	Bitrate    int // bits per second, default 64000
	Complexity int // 0-10, default 10

	// Jitter buffer
	BufferDepth int // number of frames to buffer, default 3 (60ms)

	// Voice activity detection
	VADEnabled     bool
	EnergyThreshold float64 // RMS energy threshold for speech, default 500

	// Reconnection
	Reconnect           bool
	MaxAttempts         int           // max reconnection attempts, default 10
	InitialDelay        time.Duration // first retry delay, default 1s
	MaxDelay            time.Duration // max delay cap, default 30s
	BackoffMultiplier   float64       // exponential backoff multiplier, default 2.0
	ReconnectBufferSize int           // audio frames to buffer during reconnect, default 100
}

// Validate checks the config values and returns an error if invalid.
func (c *StreamConfig) Validate() error {
	if c.Bitrate < 6000 || c.Bitrate > 512000 {
		return ErrInvalidBitrate
	}
	if c.Complexity < 0 || c.Complexity > 10 {
		return ErrInvalidComplexity
	}
	if c.BufferDepth < 1 || c.BufferDepth > 20 {
		return ErrInvalidBufferDepth
	}
	if c.EnergyThreshold <= 0 {
		return ErrInvalidEnergyThreshold
	}
	if c.MaxAttempts < 0 {
		return ErrInvalidMaxAttempts
	}
	if c.InitialDelay <= 0 {
		return ErrInvalidInitialDelay
	}
	if c.MaxDelay <= 0 {
		return ErrInvalidMaxDelay
	}
	if c.BackoffMultiplier < 1.0 {
		return ErrInvalidBackoffMultiplier
	}
	if c.ReconnectBufferSize < 0 {
		return ErrInvalidReconnectBufferSize
	}
	return nil
}

// WithBitrate sets the bitrate and returns c for chaining.
func (c *StreamConfig) WithBitrate(b int) *StreamConfig {
	c.Bitrate = b
	return c
}

// WithBufferDepth sets the buffer depth and returns c for chaining.
func (c *StreamConfig) WithBufferDepth(d int) *StreamConfig {
	c.BufferDepth = d
	return c
}

// WithVADEnabled sets VAD enabled and returns c for chaining.
func (c *StreamConfig) WithVADEnabled(enabled bool) *StreamConfig {
	c.VADEnabled = enabled
	return c
}

// WithReconnectEnabled sets reconnect enabled and returns c for chaining.
func (c *StreamConfig) WithReconnectEnabled(enabled bool) *StreamConfig {
	c.Reconnect = enabled
	return c
}
