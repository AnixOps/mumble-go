package audio

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

// UDPManager handles native UDP audio transmission.
// In Mumble, voice packets can be sent over UDP (preferred for low latency)
// instead of via TCP tunnel.
type UDPManager struct {
	mu       sync.RWMutex
	conn     net.PacketConn
	addr     net.Addr
	crypto   *CryptStateOCB2
	seq      uint32
	lastSend time.Time

	// For receiving
	decoder *OpusDecoder
	cb      AudioCallback

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewUDPManager creates a new UDP audio manager.
func NewUDPManager() *UDPManager {
	return &UDPManager{
		stopCh: make(chan struct{}),
	}
}

// SetCrypto sets the crypto state for UDP encryption/decryption.
func (u *UDPManager) SetCrypto(crypto *CryptStateOCB2) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.crypto = crypto
}

// SetDecoder sets the Opus decoder for received audio.
func (u *UDPManager) SetDecoder(dec *OpusDecoder) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.decoder = dec
}

// SetCallback sets the callback for received audio.
func (u *UDPManager) SetCallback(cb AudioCallback) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.cb = cb
}

// Dial connects to the given address.
// For Mumble, this is typically the same address as the TCP connection.
func (u *UDPManager) Dial(addr string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Mumble uses UDP on the same port as TCP
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return fmt.Errorf("udp: listen failed: %w", err)
	}

	u.conn = conn
	u.addr = &net.UDPAddr{
		IP:   net.ParseIP(addr),
		Port: 0, // Will be resolved
	}

	return nil
}

// Connect connects to a specific UDP address.
func (u *UDPManager) Connect(addr *net.UDPAddr) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return fmt.Errorf("udp: listen failed: %w", err)
	}

	u.conn = conn
	u.addr = addr

	return nil
}

// Close closes the UDP connection.
func (u *UDPManager) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	close(u.stopCh)
	u.wg.Wait()

	if u.conn != nil {
		return u.conn.Close()
	}
	return nil
}

// SendVoicePacket sends an encrypted voice packet via UDP.
func (u *UDPManager) SendVoicePacket(target uint8, sequence uint64, opusData []byte) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.conn == nil {
		return fmt.Errorf("udp: conn is nil")
	}
	if u.addr == nil {
		return fmt.Errorf("udp: addr is nil")
	}
	if u.crypto == nil {
		return fmt.Errorf("udp: crypto is nil")
	}

	// Build voice packet:
	// [header byte: type(3bits) | target(5bits)]
	// [sequence: varint]
	// [opus frames...]
	header := byte(AudioTypeOpus<<5) | (target & 0x1F)
	seqBytes := encodeVarint(sequence)

	// Build the plaintext packet
	packet := make([]byte, 0, 1+len(seqBytes)+len(opusData))
	packet = append(packet, header)
	packet = append(packet, seqBytes...)
	packet = append(packet, opusData...)

	// Encrypt the packet
	encrypted, err := u.crypto.Encrypt(packet)
	if err != nil {
		return fmt.Errorf("udp: encrypt failed: %w", err)
	}

	// Send to server
	_, err = u.conn.WriteTo(encrypted, u.addr)
	return err
}

// ReceiveLoop starts the UDP receive loop.
// It runs in a background goroutine and calls the audio callback for each received packet.
func (u *UDPManager) ReceiveLoop() {
	u.mu.RLock()
	conn := u.conn
	crypto := u.crypto
	decoder := u.decoder
	cb := u.cb
	u.mu.RUnlock()

	if conn == nil || crypto == nil {
		return
	}

	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-u.stopCh:
				return
			default:
			}
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, _, err := conn.ReadFrom(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			// Decrypt
			plaintext, err := crypto.Decrypt(buf[:n], 0)
			if err != nil {
				continue // Skip invalid packets
			}

			// Parse voice packet
			if len(plaintext) < 2 {
				continue
			}

			header := plaintext[0]
			audioType := (header >> 5) & 0x07
			if audioType != AudioTypeOpus && audioType != AudioTypeCELTBeta && audioType != AudioTypeSpeex {
				continue
			}

			// Skip header, decode varint sequence
			data := plaintext[1:]
			seq, n := decodeVarint(data)
			if n == 0 {
				continue
			}
			data = data[n:]

			// Decode opus frames
			for len(data) > 0 {
				frameSize, n := decodeVarint(data)
				if n == 0 {
					break
				}
				data = data[n:]

				terminator := (frameSize & 0x2000) != 0
				frameSize &= 0x1FFF

				if int(frameSize) <= len(data) && frameSize > 0 && decoder != nil && cb != nil {
					pcm, err := decoder.Decode(data[:frameSize])
					if err == nil {
						cb(0, seq, pcm) // Session unknown from UDP
					}
				}

				data = data[frameSize:]
				if terminator {
					break
				}
				seq += uint64(FrameDurationMs / SequenceDuration)
			}
		}
	}()
}

// SendPing sends a UDP ping packet to keep the connection alive.
// Returns the round-trip time.
func (u *UDPManager) SendPing() (time.Duration, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.conn == nil || u.addr == nil {
		return 0, fmt.Errorf("udp: not connected")
	}

	// Build ping packet: type=1 (PING), target=0
	// [header byte: type=1 | target=0] = 0x20
	packet := []byte{0x20}

	// Add timestamp
	timestamp := uint64(time.Now().UnixNano())
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, timestamp)
	packet = append(packet, timestampBytes...)

	// Encrypt
	encrypted, err := u.crypto.Encrypt(packet)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	_, err = u.conn.WriteTo(encrypted, u.addr)
	if err != nil {
		return 0, err
	}

	// Wait for pong (same packet comes back)
	buf := make([]byte, 4096)
	u.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, _, err := u.conn.ReadFrom(buf)
	if err != nil {
		return 0, err
	}

	rtt := time.Since(start)

	// Verify it's the same packet coming back
	plaintext, err := u.crypto.Decrypt(buf[:n], 0)
	if err != nil {
		return 0, err
	}

	if len(plaintext) < 9 || plaintext[0] != 0x20 {
		return 0, fmt.Errorf("udp: unexpected pong packet")
	}

	return rtt, nil
}

// LocalAddr returns the local UDP address.
func (u *UDPManager) LocalAddr() net.Addr {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if u.conn == nil {
		return nil
	}
	return u.conn.LocalAddr()
}
