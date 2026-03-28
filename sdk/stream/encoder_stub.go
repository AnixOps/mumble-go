//go:build !opus

package stream

import "fmt"

// OpusEncoder is a stub when opus support is not compiled in.
type OpusEncoder struct{}

// NewOpusEncoder returns an error when opus support is not available.
func NewOpusEncoder(sampleRate, channels, frameSize, bitrate int) (*OpusEncoder, error) {
	return nil, fmt.Errorf("opus encoder: not available (build with -tags opus)")
}

// Encode returns an error.
func (e *OpusEncoder) Encode(pcm []byte) ([]byte, error) {
	return nil, fmt.Errorf("opus encoder: not available")
}

// SetBitrate returns an error.
func (e *OpusEncoder) SetBitrate(bitrate int) error {
	return fmt.Errorf("opus encoder: not available")
}

// SetComplexity returns an error.
func (e *OpusEncoder) SetComplexity(complexity int) error {
	return fmt.Errorf("opus encoder: not available")
}

// Bitrate returns 0.
func (e *OpusEncoder) Bitrate() int {
	return 0
}

// FrameSize returns 0.
func (e *OpusEncoder) FrameSize() int {
	return 0
}

// Close does nothing.
func (e *OpusEncoder) Close() error {
	return nil
}
