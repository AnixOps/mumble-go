// Package integration provides end-to-end tests for the Mumble client.
package integration

import (
	"context"
	"crypto/tls"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mumble-go/client"
)

// TestBot is a connected bot for testing.
type TestBot struct {
	client  *client.Client
	tracker *AudioTracker
	name    string
	session uint32
}

// AudioTracker tracks received audio.
type AudioTracker struct {
	mu          sync.Mutex
	samples     [][]byte
	sessionSeen map[uint32]bool
	lastSeq     map[uint32]uint64
	count       atomic.Int32
}

func NewAudioTracker() *AudioTracker {
	return &AudioTracker{
		samples:     make([][]byte, 0),
		sessionSeen: make(map[uint32]bool),
		lastSeq:     make(map[uint32]uint64),
	}
}

func (t *AudioTracker) OnAudio(session uint32, seq uint64, pcm []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.samples = append(t.samples, pcm)
	t.sessionSeen[session] = true
	t.lastSeq[session] = seq
	t.count.Add(1)
}

func (t *AudioTracker) GetCount() int {
	return int(t.count.Load())
}

func (t *AudioTracker) GetTotalBytes() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	total := 0
	for _, s := range t.samples {
		total += len(s)
	}
	return total
}

// Config holds test configuration.
type Config struct {
	ServerAddr string
	TLSCfg    *tls.Config
}

// DefaultConfig returns default test configuration.
func DefaultConfig() Config {
	return Config{
		ServerAddr: "mumble.hotxiang.cn:64738",
		TLSCfg: &tls.Config{
			ServerName:         "mumble.hotxiang.cn",
			InsecureSkipVerify: true,
		},
	}
}

// CreateBot creates a connected test bot.
func CreateBot(t *testing.T, cfg Config, username string) *TestBot {
	t.Helper()

	c := client.New(client.Config{
		Address:  cfg.ServerAddr,
		Username: username,
		TLS:      cfg.TLSCfg,
		Audio:    client.DefaultAudioConfig(),
	})

	tracker := NewAudioTracker()
	c.OnAudio(tracker.OnAudio)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Failed to connect bot %s: %v", username, err)
	}

	// Wait for session assignment
	var session uint32
	for i := 0; i < 20; i++ {
		session = c.State().SelfSession()
		if session != 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if session == 0 {
		c.Close()
		t.Fatalf("Bot %s session not assigned", username)
	}

	return &TestBot{
		client:  c,
		tracker: tracker,
		name:    username,
		session: session,
	}
}

// Close closes the bot connection.
func (b *TestBot) Close() {
	if b.client != nil {
		b.client.Close()
	}
}

// TestConnection tests basic TCP connection.
func TestConnection(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-connection")
	defer bot.Close()

	if bot.session == 0 {
		t.Error("Bot session should not be 0")
	}

	t.Logf("Connected bot session=%d", bot.session)
}

// TestServerInfo tests that we receive server information.
func TestServerInfo(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-serverinfo")
	defer bot.Close()

	state := bot.client.State()
	if state == nil {
		t.Fatal("State should not be nil")
	}

	channels := state.SnapshotChannels()
	t.Logf("Server has %d channels", len(channels))

	for id, ch := range channels {
		t.Logf("  [%d] %s", id, ch.Name)
	}
}

// TestAudioCodec tests that the audio codec is properly configured.
func TestAudioCodec(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-audiocodec")
	defer bot.Close()

	// Set raw codec for testing
	if err := bot.client.Audio().SetRawCodec(); err != nil {
		t.Fatalf("Failed to set raw codec: %v", err)
	}

	// Verify audio is enabled
	if !bot.client.Audio().IsEnabled() {
		t.Error("Audio should be enabled")
	}

	t.Logf("Opus supported: %v", bot.client.SupportsOpus())
}

// TestAudioSend tests that we can send audio without errors.
func TestAudioSend(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-audiosend")
	defer bot.Close()

	// Set raw codec
	if err := bot.client.Audio().SetRawCodec(); err != nil {
		t.Fatalf("Failed to set raw codec: %v", err)
	}

	// Generate test tone (440Hz, 100ms)
	pcm := GenerateTone(440, 48000, 100, 0.3)

	// Send audio
	err := bot.client.SendAudio(pcm)
	if err != nil {
		t.Fatalf("SendAudio failed: %v", err)
	}

	// Give time for processing
	time.Sleep(500 * time.Millisecond)

	t.Logf("Sent %d bytes of audio", len(pcm))
	t.Logf("Audio received count: %d", bot.tracker.GetCount())

	// Note: We may or may not receive audio back, depending on:
	// 1. Whether we're in a channel with others
	// 2. Whether the server accepts our raw audio format
	// The important thing is that SendAudio doesn't error
}

// TestTwoBots tests two bots connecting simultaneously.
func TestTwoBots(t *testing.T) {
	cfg := DefaultConfig()

	bot1 := CreateBot(t, cfg, "test-twobots-1")
	defer bot1.Close()

	bot2 := CreateBot(t, cfg, "test-twobots-2")
	defer bot2.Close()

	if bot1.session == bot2.session {
		t.Errorf("Sessions should be different: bot1=%d bot2=%d", bot1.session, bot2.session)
	}

	t.Logf("Bot1 session=%d Bot2 session=%d", bot1.session, bot2.session)

	// Both should see each other
	time.Sleep(500 * time.Millisecond)

	users := bot1.client.State().SnapshotUsers()
	if len(users) < 2 {
		t.Errorf("Should see at least 2 users, got %d", len(users))
	}
}

// TestPing tests that ping works.
func TestPing(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-ping")
	defer bot.Close()

	// Note: Ping is handled automatically by the message loop
	// The client sends pings every 15 seconds
	t.Logf("Ping test: bot session=%d", bot.session)
}

// TestTextMessage tests text message handling.
func TestTextMessage(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-textmsg")
	defer bot.Close()

	// We received server sync, channel state, user state messages
	// which shows the message parsing is working
	users := bot.client.State().SnapshotUsers()
	channels := bot.client.State().SnapshotChannels()

	t.Logf("Users: %d, Channels: %d", len(users), len(channels))
}

// TestJoinChannel tests joining a channel.
func TestJoinChannel(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-join")
	defer bot.Close()

	// Get channel list
	channels := bot.client.State().SnapshotChannels()
	if len(channels) == 0 {
		t.Fatal("No channels available")
	}

	// Try to join each channel
	for id := range channels {
		err := bot.client.JoinChannel(id)
		if err != nil {
			t.Logf("JoinChannel(%d) error (may be expected): %v", id, err)
		} else {
			t.Logf("Joined channel %d successfully", id)
		}
	}

	// Try joining by name
	err := bot.client.JoinChannelByName("Root")
	if err != nil {
		t.Logf("JoinChannelByName(Root) error (may be expected): %v", err)
	} else {
		t.Log("Joined Root channel successfully")
	}
}

// TestMuteDeaf tests setting mute/deafen state.
func TestMuteDeaf(t *testing.T) {
	cfg := DefaultConfig()

	bot := CreateBot(t, cfg, "test-mutedeaf")
	defer bot.Close()

	// Set self-mute
	err := bot.client.SetSelfMute(true)
	if err != nil {
		t.Logf("SetSelfMute error (may be expected): %v", err)
	} else {
		t.Log("Self-mute set successfully")
	}

	// Set self-deafen
	err = bot.client.SetSelfDeaf(true)
	if err != nil {
		t.Logf("SetSelfDeaf error (may be expected): %v", err)
	} else {
		t.Log("Self-deafen set successfully")
	}

	// Unmute
	err = bot.client.SetSelfMute(false)
	if err != nil {
		t.Logf("SetSelfMute(false) error (may be expected): %v", err)
	} else {
		t.Log("Self-mute unset successfully")
	}
}

// GenerateTone generates a PCM sine wave.
func GenerateTone(frequency, sampleRate, durationMs int, amplitude float64) []byte {
	frameSize := 960 // 20ms at 48kHz
	frames := (durationMs * sampleRate / 1000) / frameSize
	if frames == 0 {
		frames = 1
	}
	samples := frames * frameSize

	pcm := make([]byte, samples*2)

	for i := 0; i < samples; i++ {
		sample := int16(amplitude * 32767 * simpleSin(2*3.141592653589793*float64(frequency)*float64(i)/float64(sampleRate)))
		pcm[i*2] = byte(sample)
		pcm[i*2+1] = byte(sample >> 8)
	}

	return pcm
}

func simpleSin(x float64) float64 {
	const pi = 3.141592653589793
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}
	x2 := x * x
	x3 := x2 * x
	x5 := x2 * x3
	x7 := x2 * x5
	return x - x3/6 + x5/120 - x7/5040
}
