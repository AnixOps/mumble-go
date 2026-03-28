package audio

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// AudioCallback is called when audio is received from a user.
type AudioCallback func(session uint32, sequence uint64, pcm []byte)

// Input handles audio decoding and reception.
type Input struct {
	mu sync.Mutex

	decoder  *OpusDecoder
	crypto   *CryptStateOCB2
	callback AudioCallback

	// User audio buffers for reassembly
	userBuffers map[uint32]*userBuffer
}

type userBuffer struct {
	lastSequence uint64
	lastTime     time.Time
}

// NewInput creates a new audio input handler.
func NewInput() *Input {
	return &Input{
		userBuffers: make(map[uint32]*userBuffer),
	}
}

// SetDecoder sets the Opus decoder.
func (i *Input) SetDecoder(dec *OpusDecoder) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.decoder = dec
}

// SetCrypto sets the crypto state for UDP decryption.
func (i *Input) SetCrypto(crypto *CryptStateOCB2) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.crypto = crypto
}

// SetCallback sets the callback for received audio.
func (i *Input) SetCallback(cb AudioCallback) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.callback = cb
}

// GetCallback returns the current callback.
func (i *Input) GetCallback() AudioCallback {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.callback
}

// ProcessPacket processes an incoming audio packet (from UDPTunnel or UDP).
// The packet format is:
// [header byte: type(3bits) | target(5bits)]
// [varint: session]
// [varint: sequence]
// [opus frames...]
func (i *Input) ProcessPacket(data []byte) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if len(data) < 1 {
		return fmt.Errorf("audio: empty packet")
	}

	// Parse header
	header := data[0]
	audioType := (header >> 5) & 0x07
	log.Printf("[audio] ProcessPacket: header=0x%02x audioType=%d len=%d", header, audioType, len(data))
	// target := header & 0x1F

	if audioType == 1 { // PING
		return nil // Ignore ping packets
	}

	if audioType != AudioTypeOpus && audioType != AudioTypeCELTBeta && audioType != AudioTypeSpeex {
		return fmt.Errorf("audio: unsupported codec type %d", audioType)
	}

	data = data[1:]

	// Parse session (varint)
	session, n := decodeVarint(data)
	if n == 0 {
		return fmt.Errorf("audio: invalid session")
	}
	data = data[n:]

	// Parse sequence (varint)
	sequence, n := decodeVarint(data)
	if n == 0 {
		return fmt.Errorf("audio: invalid sequence")
	}
	data = data[n:]

	// Parse and decode Opus frames
	for len(data) > 0 {
		// Opus frame size (varint)
		// Bit 13 (0x2000) indicates terminator
		frameSize, n := decodeVarint(data)
		if n == 0 {
			break
		}
		data = data[n:]

		terminator := (frameSize & 0x2000) != 0
		frameSize &= 0x1FFF

		if int(frameSize) > len(data) {
			return fmt.Errorf("audio: frame size %d exceeds data length %d", frameSize, len(data))
		}

		if frameSize > 0 && i.decoder != nil {
			opusData := data[:frameSize]
			pcm, err := i.decoder.Decode(opusData)
			if err != nil {
				log.Printf("[audio] decode error: %v", err)
			} else if i.callback != nil {
				i.callback(uint32(session), sequence, pcm)
			}
		}

		data = data[frameSize:]

		if terminator {
			break
		}

		// Increment sequence for each 10ms of audio
		sequence += FrameDurationMs / SequenceDuration
	}

	return nil
}

// decodeVarint decodes a varint from a byte slice.
// Returns the value and the number of bytes consumed.
func decodeVarint(data []byte) (uint64, int) {
	var result uint64
	var shift uint

	for i, b := range data {
		result |= uint64(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			return result, i + 1
		}
		if shift >= 64 {
			return 0, 0
		}
	}

	return result, 0
}

// ProcessEncrypted processes an encrypted UDP audio packet.
func (i *Input) ProcessEncrypted(data []byte, plainLen int) error {
	i.mu.Lock()
	crypto := i.crypto
	i.mu.Unlock()

	if crypto == nil {
		return fmt.Errorf("audio: no crypto state for decryption")
	}

	decrypted, err := crypto.Decrypt(data, plainLen)
	if err != nil {
		return fmt.Errorf("audio: decrypt failed: %w", err)
	}

	return i.ProcessPacket(decrypted)
}
