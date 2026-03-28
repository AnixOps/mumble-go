//go:build opus
// +build opus

package client

import (
	"fmt"

	"mumble-go/audio"
)

// SetOpusCodec sets the real Opus encoder and decoder using libopus.
func (a *Audio) SetOpusCodec() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	opusEnc, err := audio.NewOpusEnc(audio.SampleRate, audio.Channels, audio.FrameSize)
	if err != nil {
		return fmt.Errorf("opus encoder: %w", err)
	}

	opusDec, err := audio.NewOpusDec(audio.SampleRate, audio.Channels, audio.FrameSize)
	if err != nil {
		return fmt.Errorf("opus decoder: %w", err)
	}

	a.encoder, err = audio.NewOpusEncoder(opusEnc)
	if err != nil {
		return err
	}

	a.decoder, err = audio.NewOpusDecoder(opusDec)
	if err != nil {
		return err
	}

	a.output.SetEncoder(a.encoder)
	a.input.SetDecoder(a.decoder)

	return nil
}
