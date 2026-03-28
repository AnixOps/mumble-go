package audio

import (
	"fmt"
)

// Encoder is the interface for audio encoders.
type Encoder interface {
	Encode(pcm []byte) ([]byte, error)
	SetBitrate(bitrate int) error
	FrameSize() int
}

// Decoder is the interface for audio decoders.
type Decoder interface {
	Decode(data []byte) ([]byte, error)
	FrameSize() int
}

// OpusEncoder wraps an Opus encoder implementation.
type OpusEncoder struct {
	encoder   Encoder
	frameSize int
}

// NewOpusEncoder creates a new Opus encoder wrapper.
func NewOpusEncoder(enc Encoder) (*OpusEncoder, error) {
	if enc == nil {
		return nil, fmt.Errorf("opus: encoder is nil")
	}
	return &OpusEncoder{
		encoder:   enc,
		frameSize: enc.FrameSize(),
	}, nil
}

// Encode encodes PCM data to Opus.
func (e *OpusEncoder) Encode(pcm []byte) ([]byte, error) {
	return e.encoder.Encode(pcm)
}

// SetBitrate sets the encoding bitrate.
func (e *OpusEncoder) SetBitrate(bitrate int) error {
	return e.encoder.SetBitrate(bitrate)
}

// FrameSize returns the frame size in samples.
func (e *OpusEncoder) FrameSize() int {
	return e.frameSize
}

// OpusDecoder wraps an Opus decoder implementation.
type OpusDecoder struct {
	decoder   Decoder
	frameSize int
}

// NewOpusDecoder creates a new Opus decoder wrapper.
func NewOpusDecoder(dec Decoder) (*OpusDecoder, error) {
	if dec == nil {
		return nil, fmt.Errorf("opus: decoder is nil")
	}
	return &OpusDecoder{
		decoder:   dec,
		frameSize: dec.FrameSize(),
	}, nil
}

// Decode decodes Opus data to PCM.
func (d *OpusDecoder) Decode(data []byte) ([]byte, error) {
	return d.decoder.Decode(data)
}

// FrameSize returns the frame size in samples.
func (d *OpusDecoder) FrameSize() int {
	return d.frameSize
}
