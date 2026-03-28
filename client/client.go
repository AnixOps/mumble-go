package client

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"mumble-go/audio"
	"mumble-go/identity"
	"mumble-go/protocol"
	"mumble-go/state"
	"mumble-go/transport"
)

// Config holds connection settings for the Mumble client.
type Config struct {
	Address  string
	Username string
	Password string
	Tokens   []string
	TLS      *tls.Config
	Identity identity.Provider

	// Audio configuration
	Audio AudioConfig
}

// Client is the high-level headless Mumble client.
type Client struct {
	cfg   Config
	conn  *transport.Conn
	store *state.Store

	// Audio subsystem
	audio      *Audio
	udpManager *audio.UDPManager

	// Event handler
	events *EventHandler

	// Crypto state for audio (populated after CryptSetup)
	cryptoKey         []byte
	cryptoServerNonce []byte
	cryptoClientNonce []byte
	codecOpus         bool

	// Runtime state
	closeOnce sync.Once
	closeCh   chan struct{}
	wg        sync.WaitGroup
}

// New creates a new Mumble client with the given configuration.
func New(cfg Config) *Client {
	// Initialize audio (without codec - user must call SetCodec)
	audioConfig := cfg.Audio
	if audioConfig.Bandwidth == 0 {
		audioConfig = DefaultAudioConfig()
	}
	audioHandler := NewAudio(audioConfig)

	return &Client{
		cfg:     cfg,
		store:   state.NewStore(),
		audio:   audioHandler,
		events:  NewEventHandler(),
		closeCh: make(chan struct{}),
	}
}

// Events returns the event handler for registering callbacks.
func (c *Client) Events() *EventHandler {
	return c.events
}

func (c *Client) dial() error {
	if c.cfg.Address == "" {
		return fmt.Errorf("client: empty address")
	}

	tlsCfg := c.cfg.TLS
	if tlsCfg == nil {
		tlsCfg = &tls.Config{}
	} else {
		tlsCfg = tlsCfg.Clone()
	}
	if c.cfg.Identity != nil {
		cert, err := c.cfg.Identity.TLSCertificate()
		if err != nil {
			return fmt.Errorf("client: load identity: %w", err)
		}
		if cert != nil {
			tlsCfg.Certificates = append([]tls.Certificate{*cert}, tlsCfg.Certificates...)
		}
	}

	conn, err := transport.Dial(c.cfg.Address, tlsCfg)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Close cleanly shuts down the client connection.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		close(c.closeCh)
	})
	c.wg.Wait()
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) State() *state.Store { return c.store }

// SupportsOpus returns true if the server supports Opus audio codec.
func (c *Client) SupportsOpus() bool { return c.codecOpus }

// Audio returns the audio handler for sending/receiving audio.
func (c *Client) Audio() *Audio { return c.audio }

// SendAudio queues and sends PCM audio data.
// Data must be 16-bit signed PCM at 48kHz mono.
func (c *Client) SendAudio(pcm []byte) error {
	if c.audio == nil {
		return fmt.Errorf("client: audio not initialized")
	}
	c.audio.SendPCM(pcm)
	_, err := c.audio.Send()
	return err
}

// SendAudioUDP encodes and sends PCM audio via UDP.
// This is an alternative to TCP tunnel audio transmission.
func (c *Client) SendAudioUDP(pcm []byte) error {
	if c.audio == nil {
		return fmt.Errorf("client: audio not initialized")
	}
	if c.udpManager == nil {
		return fmt.Errorf("client: UDP not initialized")
	}

	// Add PCM to buffer and get encoded frames
	c.audio.SendPCM(pcm)
	frames, err := c.audio.Output().EncodeFrames()
	if err != nil {
		return fmt.Errorf("encode frames: %w", err)
	}
	if len(frames) == 0 {
		return nil
	}

	// Get starting sequence
	seq := c.audio.Output().AdvanceSequence(len(frames))

	// Send each frame individually via UDP
	frameDuration := audio.FrameDurationMs / audio.SequenceDuration
	for i, frame := range frames {
		frameSeq := seq + uint32(i)*uint32(frameDuration)
		if err := c.udpManager.SendVoicePacket(0, uint64(frameSeq), frame); err != nil {
			return fmt.Errorf("send frame %d: %w", i, err)
		}
	}

	return nil
}

// OnAudio sets the callback for received audio.
func (c *Client) OnAudio(cb audio.AudioCallback) {
	if c.audio != nil {
		c.audio.SetCallback(cb)
	}
}

// Connected returns true if the client has an active connection.
func (c *Client) Connected() bool {
	return c.conn != nil
}

// SendUserState sends a raw UserState protobuf payload.
func (c *Client) SendUserState(payload []byte) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}
	return c.conn.WriteFrame(protocol.MessageTypeUserState, payload)
}

// JoinChannel moves the client to the specified channel.
func (c *Client) JoinChannel(channelID uint32) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	session := c.store.SelfSession()
	if session == 0 {
		return fmt.Errorf("client: session not assigned")
	}

	us := &protocol.UserState{
		Session:   session,
		ChannelID: channelID,
	}

	payload, err := us.Marshal()
	if err != nil {
		return fmt.Errorf("join channel: marshal failed: %w", err)
	}

	return c.conn.WriteFrame(protocol.MessageTypeUserState, payload)
}

// JoinChannelByName moves the client to the channel with the given name.
func (c *Client) JoinChannelByName(name string) error {
	channels := c.store.SnapshotChannels()
	for id, ch := range channels {
		if ch.Name == name {
			return c.JoinChannel(id)
		}
	}
	return fmt.Errorf("client: channel '%s' not found", name)
}

// SelfRegister asks the server to register the current user identity.
// This matches the official client behavior: UserState{session, user_id=0}.
func (c *Client) SelfRegister() error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	session := c.store.SelfSession()
	if session == 0 {
		return fmt.Errorf("client: session not assigned")
	}

	us := &protocol.UserState{
		Session:       session,
		UserID:        0,
		IncludeUserID: true,
	}

	payload, err := us.Marshal()
	if err != nil {
		return fmt.Errorf("self register: marshal failed: %w", err)
	}

	return c.conn.WriteFrame(protocol.MessageTypeUserState, payload)
}

// SendMessage sends a text message to specified sessions.
func (c *Client) SendMessage(message string, sessions []uint32) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	session := c.store.SelfSession()
	tm := &protocol.TextMessage{
		Actor:   session,
		Session: sessions,
		Message: message,
	}

	payload, err := tm.Marshal()
	if err != nil {
		return fmt.Errorf("send message: marshal failed: %w", err)
	}

	return c.conn.WriteFrame(protocol.MessageTypeTextMessage, payload)
}

// SendChannelMessage sends a text message to a channel.
func (c *Client) SendChannelMessage(channelID uint32, message string) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	session := c.store.SelfSession()
	tm := &protocol.TextMessage{
		Actor:     session,
		ChannelID: []uint32{channelID},
		Message:   message,
	}

	payload, err := tm.Marshal()
	if err != nil {
		return fmt.Errorf("send channel message: marshal failed: %w", err)
	}

	return c.conn.WriteFrame(protocol.MessageTypeTextMessage, payload)
}

// SetSelfMute sets the client's self-mute state.
func (c *Client) SetSelfMute(muted bool) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	session := c.store.SelfSession()
	if session == 0 {
		return fmt.Errorf("client: session not assigned")
	}

	us := &protocol.UserState{
		Session:  session,
		SelfMute: muted,
	}

	payload, err := us.Marshal()
	if err != nil {
		return fmt.Errorf("set self mute: marshal failed: %w", err)
	}

	return c.conn.WriteFrame(protocol.MessageTypeUserState, payload)
}

// SetSelfDeaf sets the client's self-deafen state.
func (c *Client) SetSelfDeaf(deaf bool) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	session := c.store.SelfSession()
	if session == 0 {
		return fmt.Errorf("client: session not assigned")
	}

	us := &protocol.UserState{
		Session:  session,
		SelfDeaf: deaf,
	}

	payload, err := us.Marshal()
	if err != nil {
		return fmt.Errorf("set self deaf: marshal failed: %w", err)
	}

	return c.conn.WriteFrame(protocol.MessageTypeUserState, payload)
}

// generateNonce creates a 24-byte random nonce for OCB-AES.
func generateNonce() ([]byte, error) {
	b := make([]byte, 24)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return b, nil
}

// SetupUDP initializes the UDP audio manager for native UDP audio transmission.
// Returns the local UDP port that was opened.
func (c *Client) SetupUDP() (int, error) {
	// Parse host and port from address
	host, portStr, err := splitHostPort(c.cfg.Address)
	if err != nil {
		return 0, fmt.Errorf("setup udp: parse address: %w", err)
	}

	addr, err := net.ResolveUDPAddr("udp", host+":"+portStr)
	if err != nil {
		return 0, fmt.Errorf("setup udp: resolve address: %w", err)
	}

	c.udpManager = audio.NewUDPManager()
	c.udpManager.SetDecoder(c.audio.decoder)
	c.udpManager.SetCallback(c.audio.GetCallback())

	if err := c.udpManager.Connect(addr); err != nil {
		return 0, fmt.Errorf("setup udp: connect: %w", err)
	}

	// Set crypto if available
	fmt.Printf("[setup] cryptoKey len=%d, clientNonce len=%d, serverNonce len=%d\n",
		len(c.cryptoKey), len(c.cryptoClientNonce), len(c.cryptoServerNonce))

	if len(c.cryptoKey) >= 16 && len(c.cryptoClientNonce) >= 16 && len(c.cryptoServerNonce) >= 16 {
		crypto := audio.NewCryptStateOCB2()
		if err := crypto.SetKey(c.cryptoKey[:16], c.cryptoClientNonce[:16], c.cryptoServerNonce[:16]); err != nil {
			fmt.Printf("[setup] SetKey failed: %v\n", err)
		} else {
			c.udpManager.SetCrypto(crypto)
			fmt.Println("[setup] UDP crypto set successfully")
		}
	} else {
		fmt.Println("[setup] Crypto keys not available")
	}

	// Start receive loop
	c.udpManager.ReceiveLoop()

	localAddr := c.udpManager.LocalAddr()
	if udpAddr, ok := localAddr.(*net.UDPAddr); ok {
		return udpAddr.Port, nil
	}
	return 0, nil
}

// UDPManager returns the UDP manager if initialized.
func (c *Client) UDPManager() *audio.UDPManager {
	return c.udpManager
}

// splitHostPort splits a host:port string.
func splitHostPort(addr string) (string, string, error) {
	// Simple implementation
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			// Check if it's an IPv6 address
			if addr[0] == '[' {
				continue
			}
			return addr[:i], addr[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid address: %s", addr)
}
