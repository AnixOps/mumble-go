//go:build opus
// +build opus

package audio

import (
	"encoding/binary"
	"fmt"

	"github.com/hraban/opus"
)

// OpusEnc is a wrapper around hraban/opus.Encoder.
type OpusEnc struct {
	enc      *opus.Encoder
	frameSize int
}

// NewOpusEnc creates a new Opus encoder.
func NewOpusEnc(sampleRate, channels, frameSize int) (*OpusEnc, error) {
	enc, err := opus.NewEncoder(sampleRate, channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("opus encoder: %w", err)
	}
	return &OpusEnc{
		enc:      enc,
		frameSize: frameSize,
	}, nil
}

// Encode encodes PCM data to Opus.
func (e *OpusEnc) Encode(pcm []byte) ([]byte, error) {
	// Convert []byte to []int16
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

// SetBitrate sets the encoding bitrate.
func (e *OpusEnc) SetBitrate(bitrate int) error {
	return e.enc.SetBitrate(bitrate)
}

// FrameSize returns the frame size in samples.
func (e *OpusEnc) FrameSize() int {
	return e.frameSize
}

// OpusDec is a wrapper around hraban/opus.Decoder.
type OpusDec struct {
	dec      *opus.Decoder
	frameSize int
}

// NewOpusDec creates a new Opus decoder.
func NewOpusDec(sampleRate, channels, frameSize int) (*OpusDec, error) {
	dec, err := opus.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, fmt.Errorf("opus decoder: %w", err)
	}
	return &OpusDec{
		dec:      dec,
		frameSize: frameSize,
	}, nil
}

// Decode decodes Opus data to PCM.
func (d *OpusDec) Decode(data []byte) ([]byte, error) {
	pcm := make([]int16, d.frameSize*2) // frameSize * channels
	n, err := d.dec.Decode(data, pcm)
	if err != nil {
		return nil, fmt.Errorf("opus decode: %w", err)
	}

	// Convert []int16 to []byte
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(pcm[i]))
	}
	return out, nil
}

// FrameSize returns the frame size in samples.
func (d *OpusDec) FrameSize() int {
	return d.frameSize
}
