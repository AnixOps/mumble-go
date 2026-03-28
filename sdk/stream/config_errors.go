package stream

import "errors"

var (
	ErrInvalidBitrate              = errors.New("stream: bitrate must be between 6000 and 512000")
	ErrInvalidComplexity           = errors.New("stream: complexity must be between 0 and 10")
	ErrInvalidBufferDepth          = errors.New("stream: bufferDepth must be between 1 and 20")
	ErrInvalidEnergyThreshold      = errors.New("stream: energyThreshold must be positive")
	ErrInvalidMaxAttempts          = errors.New("stream: maxAttempts must be non-negative")
	ErrInvalidInitialDelay         = errors.New("stream: initialDelay must be positive")
	ErrInvalidMaxDelay             = errors.New("stream: maxDelay must be positive")
	ErrInvalidBackoffMultiplier    = errors.New("stream: backoffMultiplier must be at least 1.0")
	ErrInvalidReconnectBufferSize  = errors.New("stream: reconnectBufferSize must be non-negative")
)
