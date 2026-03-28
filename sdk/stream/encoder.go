//go:build opus

package stream

import (
	"encoding/binary"
	"fmt"

	"github.com/hraban/opus"
)

// OpusEncoder provides direct Opus encoding for the streaming sender.
// It wraps github.com/hraban/opus to encode PCM data.
type OpusEncoder struct {
	enc       *opus.Encoder
	frameSize int
	bitrate   int
}

// NewOpusEncoder creates a new Opus encoder for streaming.
func NewOpusEncoder(sampleRate, channels, frameSize, bitrate int) (*OpusEncoder, error) {
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("opus encoder: %w", err)
	}

	if err := enc.SetBitrate(bitrate); err != nil {
		return nil, fmt.Errorf("set bitrate: %w", err)
	}

	return &OpusEncoder{
		enc:       enc,
		frameSize: frameSize,
		bitrate:   bitrate,
	}, nil
}

// Encode encodes PCM data to Opus.
// Input PCM must be 16-bit signed LE, 48kHz mono.
func (e *OpusEncoder) Encode(pcm []byte) ([]byte, error) {
	int16s := make([]int16, len(pcm)/2)
	for i := range int16s {
		int16s[i] = int16(binary.LittleEndian.Uint16(pcm[i*2:]))
	}

	buf := make([]byte, 4000) // Max opus packet size
	n, err := e.enc.Encode(int16s, buf)
	if err != nil {
		return nil, fmt.Errorf("opus encode: %w", err)
	}
	return buf[:n], nil
}

// SetBitrate updates the encoding bitrate.
func (e *OpusEncoder) SetBitrate(bitrate int) error {
	if err := e.enc.SetBitrate(bitrate); err != nil {
		return err
	}
	e.bitrate = bitrate
	return nil
}

// SetComplexity updates the Opus encoder complexity (0-10).
func (e *OpusEncoder) SetComplexity(complexity int) error {
	return e.enc.SetComplexity(complexity)
}

// Bitrate returns the current encoding bitrate.
func (e *OpusEncoder) Bitrate() int {
	return e.bitrate
}

// FrameSize returns the frame size in samples.
func (e *OpusEncoder) FrameSize() int {
	return e.frameSize
}

// Close releases encoder resources.
func (e *OpusEncoder) Close() error {
	// opus.Encoder doesn't have a Close method
	return nil
}
