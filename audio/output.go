package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"mumble-go/protocol"
)

// VoicePacket represents an outgoing voice packet.
type VoicePacket struct {
	OpusData []byte
	Target   uint8 // 0=normal, 1=channel whisper, 2=user whisper
}

// Output handles audio encoding and transmission.
type Output struct {
	mu sync.Mutex

	encoder   *OpusEncoder
	crypto    *CryptStateOCB2
	writer    io.Writer // connection writer for TCP tunnel

	// Sequencing
	sequence      uint32
	sequenceStart time.Time
	lastSend      time.Time
	lastSeqTime   time.Time

	// Buffer for PCM data
	pcmBuffer [][]byte

	// Configuration
	frameDuration time.Duration
	bandwidth     int

	// State
	target uint8
	session uint32 // Our session ID for sending audio
}

// NewOutput creates a new audio output handler.
func NewOutput() *Output {
	return &Output{
		frameDuration: FrameDurationMs * time.Millisecond,
		bandwidth:     50000, // 50kbps default
		target:        TargetNormal,
	}
}

// SetEncoder sets the Opus encoder.
func (o *Output) SetEncoder(enc *OpusEncoder) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.encoder = enc
}

// SetCrypto sets the crypto state for UDP encryption.
// For TCP tunnel mode, crypto can be nil.
func (o *Output) SetCrypto(crypto *CryptStateOCB2) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.crypto = crypto
}

// SetWriter sets the connection writer for sending audio.
func (o *Output) SetWriter(w io.Writer) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.writer = w
}

// SetBandwidth sets the target bitrate in bits per second.
func (o *Output) SetBandwidth(bitrate int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.bandwidth = bitrate
	if o.encoder != nil {
		// Account for overhead (~50 bytes per packet)
		overhead := int(50 * 8 / (FrameDurationMs / 1000.0))
		o.encoder.SetBitrate(bitrate - overhead)
	}
}

// SetTarget sets the voice target (0=normal, 1=channel, 2=user).
func (o *Output) SetTarget(target uint8) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.target = target
}

// SetSession sets the session ID for outgoing audio packets.
func (o *Output) SetSession(session uint32) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.session = session
}

// AddPCM adds PCM audio data to the output buffer.
// Data must be 16-bit signed PCM at 48kHz mono.
func (o *Output) AddPCM(pcm []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Split into frame-sized chunks
	frameBytes := FrameSize * 2 // 16-bit samples = 2 bytes each
	for len(pcm) >= frameBytes {
		frame := make([]byte, frameBytes)
		copy(frame, pcm[:frameBytes])
		o.pcmBuffer = append(o.pcmBuffer, frame)
		pcm = pcm[frameBytes:]
	}

	// Handle partial frame (pad with zeros)
	if len(pcm) > 0 {
		frame := make([]byte, frameBytes)
		copy(frame, pcm)
		o.pcmBuffer = append(o.pcmBuffer, frame)
	}
}

// Send sends all buffered audio data.
// Returns the number of frames sent.
func (o *Output) Send() (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.encoder == nil || o.writer == nil {
		return 0, nil
	}

	if len(o.pcmBuffer) == 0 {
		return 0, nil
	}

	now := time.Now()

	// Update sequence
	if o.sequenceStart.IsZero() || now.Sub(o.lastSend) > 5*time.Second {
		// Reset sequence after 5 second gap
		o.sequence = 0
		o.sequenceStart = now
		o.lastSend = now
		o.lastSeqTime = now
	} else if now.Sub(o.lastSend) > FrameDurationMs*2*time.Millisecond {
		// Calculate sequence after shorter gap
		o.sequence = uint32(now.Sub(o.sequenceStart) / (SequenceDuration * time.Millisecond))
		o.lastSeqTime = o.sequenceStart.Add(time.Duration(o.sequence) * SequenceDuration * time.Millisecond)
		o.lastSend = now
	} else {
		// Continuous audio - increment by frames
		o.sequence += uint32(len(o.pcmBuffer) * FrameDurationMs / SequenceDuration)
		o.lastSend = now
	}

	// Build audio packet
	packet, err := o.buildPacket()
	if err != nil {
		return 0, err
	}

	// Send via TCP tunnel
	if _, err := o.writer.Write(packet); err != nil {
		return 0, err
	}

	frames := len(o.pcmBuffer)
	o.pcmBuffer = nil

	return frames, nil
}

// buildPacket constructs a UDPTunnel audio packet.
func (o *Output) buildPacket() ([]byte, error) {
	// Audio packet format for OUTGOING (TCP tunnel):
	// [header byte: type(3bits) | target(5bits)]
	// [varint: sequence]
	// [varint: opus_frame_size]
	// [opus frame data]
	// [... more frames ...]
	//
	// NOTE: Session is NOT included in outgoing packets because the server
	// identifies the sender via the TCP connection itself.

	var payload []byte

	// Encode all frames
	for i, pcm := range o.pcmBuffer {
		opusData, err := o.encoder.Encode(pcm)
		if err != nil {
			log.Printf("[audio] encode error: %v", err)
			continue
		}

		// Opus frame header: varint size with terminator bit (bit 13) set on LAST frame
		size := len(opusData)
		isLast := (i == len(o.pcmBuffer)-1)
		if isLast {
			size |= 0x2000 // Set terminator bit on last frame
		}
		frameHeader := encodeVarint(uint64(size))
		payload = append(payload, frameHeader...)
		payload = append(payload, opusData...)
	}

	if len(payload) == 0 {
		return nil, nil
	}

	// Build complete audio packet
	header := byte(AudioTypeOpus<<5) | o.target
	seq := encodeVarint(uint64(o.sequence))

	audioPacket := make([]byte, 0, 1+len(seq)+len(payload))
	audioPacket = append(audioPacket, header)
	audioPacket = append(audioPacket, seq...)
	audioPacket = append(audioPacket, payload...)

	// Debug: log audio packet details
	if protocol.EnableDebug && len(audioPacket) > 0 {
		log.Printf("[audio] Sending audio packet: header=0x%02x seq=%d frames=%d payload_len=%d",
			header, o.sequence, len(o.pcmBuffer), len(payload))
	}

	// Wrap in UDPTunnel message
	// Format: [type:2 bytes][length:4 bytes][audio_packet]
	tunnelPacket := make([]byte, 6+len(audioPacket))
	binary.BigEndian.PutUint16(tunnelPacket[0:2], 1) // MessageTypeUDPTunnel
	binary.BigEndian.PutUint32(tunnelPacket[2:6], uint32(len(audioPacket)))
	copy(tunnelPacket[6:], audioPacket)

	return tunnelPacket, nil
}

// encodeVarint encodes a uint64 as a varint (similar to protobuf varint).
func encodeVarint(n uint64) []byte {
	var buf []byte
	for {
		b := byte(n & 0x7F)
		n >>= 7
		if n > 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if n == 0 {
			break
		}
	}
	return buf
}

// ClearBuffer clears the PCM buffer.
func (o *Output) ClearBuffer() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pcmBuffer = nil
}

// BufferDuration returns the duration of buffered audio.
func (o *Output) BufferDuration() time.Duration {
	o.mu.Lock()
	defer o.mu.Unlock()
	return time.Duration(len(o.pcmBuffer)) * FrameDurationMs * time.Millisecond
}

// GetSequence returns the current sequence number.
func (o *Output) GetSequence() uint32 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sequence
}

// EncodeFrames encodes all buffered PCM frames to Opus and returns individual frames.
// This is used for UDP transmission where we need to send frames separately.
func (o *Output) EncodeFrames() ([][]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.encoder == nil {
		return nil, fmt.Errorf("encoder not set")
	}

	if len(o.pcmBuffer) == 0 {
		return nil, nil
	}

	var encoded [][]byte
	for _, pcm := range o.pcmBuffer {
		opusData, err := o.encoder.Encode(pcm)
		if err != nil {
			log.Printf("[audio] encode error: %v", err)
			continue
		}
		encoded = append(encoded, opusData)
	}

	return encoded, nil
}
