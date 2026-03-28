package client

import (
	"io"
	"sync"
	"time"

	"mumble-go/audio"
)

// AudioConfig holds audio configuration.
type AudioConfig struct {
	Bandwidth     int  // Target bitrate in bps (default: 50000)
	EnableReceive bool // Enable receiving audio
}

// DefaultAudioConfig returns default audio configuration.
func DefaultAudioConfig() AudioConfig {
	return AudioConfig{
		Bandwidth:     50000,
		EnableReceive: true,
	}
}

// Audio handles audio I/O for the client.
type Audio struct {
	mu sync.RWMutex

	output  *audio.Output
	input   *audio.Input
	encoder *audio.OpusEncoder
	decoder *audio.OpusDecoder

	enabled   bool
	bandwidth int
}

// NewAudio creates a new Audio handler without codec (must call SetCodec).
func NewAudio(cfg AudioConfig) *Audio {
	out := audio.NewOutput()
	out.SetBandwidth(cfg.Bandwidth)

	in := audio.NewInput()

	return &Audio{
		output:   out,
		input:    in,
		bandwidth: cfg.Bandwidth,
		enabled:  true,
	}
}

// SetCodec sets the Opus encoder and decoder.
func (a *Audio) SetCodec(enc audio.Encoder, dec audio.Decoder) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var err error
	a.encoder, err = audio.NewOpusEncoder(enc)
	if err != nil {
		return err
	}

	a.decoder, err = audio.NewOpusDecoder(dec)
	if err != nil {
		return err
	}

	a.output.SetEncoder(a.encoder)
	a.input.SetDecoder(a.decoder)

	return nil
}

// SetRawCodec sets a raw passthrough encoder/decoder for testing.
// This bypasses Opus and sends raw PCM-like data.
func (a *Audio) SetRawCodec() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	rawEnc := audio.NewRawEncoder()
	rawDec := audio.NewRawDecoder()

	var err error
	a.encoder, err = audio.NewOpusEncoder(rawEnc)
	if err != nil {
		return err
	}

	a.decoder, err = audio.NewOpusDecoder(rawDec)
	if err != nil {
		return err
	}

	a.output.SetEncoder(a.encoder)
	a.input.SetDecoder(a.decoder)

	return nil
}

// SetWriter sets the connection writer for audio output.
func (a *Audio) SetWriter(w io.Writer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.output.SetWriter(w)
}

// SetCrypto sets the crypto state for audio.
func (a *Audio) SetCrypto(key, encryptIV, decryptIV []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	crypto := audio.NewCryptStateOCB2()
	if err := crypto.SetKey(key, encryptIV, decryptIV); err != nil {
		return err
	}

	a.output.SetCrypto(crypto)
	a.input.SetCrypto(crypto)

	return nil
}

// SetCallback sets the callback for received audio.
func (a *Audio) SetCallback(cb audio.AudioCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.input.SetCallback(cb)
}

// GetCallback returns the current audio callback.
func (a *Audio) GetCallback() audio.AudioCallback {
	return a.input.GetCallback()
}

// SendPCM queues PCM audio data for sending.
// Data must be 16-bit signed PCM at 48kHz mono.
func (a *Audio) SendPCM(pcm []byte) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.enabled {
		return
	}
	a.output.AddPCM(pcm)
}

// Send flushes the audio buffer and sends all queued audio.
func (a *Audio) Send() (int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.enabled {
		return 0, nil
	}
	return a.output.Send()
}

// ProcessPacket processes an incoming audio packet.
func (a *Audio) ProcessPacket(data []byte) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.input.ProcessPacket(data)
}

// ClearBuffer clears the outgoing audio buffer.
func (a *Audio) ClearBuffer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.output.ClearBuffer()
}

// BufferDuration returns the duration of buffered audio.
func (a *Audio) BufferDuration() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.output.BufferDuration()
}

// SetBandwidth sets the target bitrate.
func (a *Audio) SetBandwidth(bitrate int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bandwidth = bitrate
	a.output.SetBandwidth(bitrate)
}

// SetTarget sets the voice target.
func (a *Audio) SetTarget(target uint8) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.output.SetTarget(target)
}

// SetSession sets the session ID for outgoing audio packets.
func (a *Audio) SetSession(session uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.output.SetSession(session)
}

// SetEnabled enables or disables audio.
func (a *Audio) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// IsEnabled returns whether audio is enabled.
func (a *Audio) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// Output returns the audio output handler.
func (a *Audio) Output() *audio.Output {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.output
}
