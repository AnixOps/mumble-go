package stream

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"mumble-go/client"
	"mumble-go/protocol"
	"mumble-go/state"
)

// mockSenderClient implements SenderClient for testing.
type mockSenderClient struct {
	mu                  sync.Mutex
	sendAudioUDPCalled  int
	sendAudioCalled     int
	sendUserStateCalled int
	lastUserState       []byte
	lastUDPPayload      []byte
	udpErr              error
	tcpErr              error
	store               *state.Store
}

func (m *mockSenderClient) SendAudio(pcm []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendAudioCalled++
	return m.tcpErr
}

func (m *mockSenderClient) SendAudioUDP(pcm []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendAudioUDPCalled++
	m.lastUDPPayload = append([]byte(nil), pcm...)
	return m.udpErr
}

func (m *mockSenderClient) SendUserState(payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendUserStateCalled++
	m.lastUserState = append([]byte(nil), payload...)
	return nil
}

func (m *mockSenderClient) Audio() *client.Audio         { return nil }
func (m *mockSenderClient) Events() *client.EventHandler { return nil }
func (m *mockSenderClient) State() *state.Store          { return m.store }

// mockAudioSource implements AudioSource for testing.
type mockAudioSource struct {
	mu   sync.Mutex
	data []byte
	pos  int
}

func (m *mockAudioSource) ReadPCM(ctx context.Context, p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	copy(p, m.data[m.pos:])
	m.pos += len(p)
	if m.pos >= len(m.data) {
		return len(p), io.EOF
	}
	return len(p), nil
}

type mockStreamEncoder struct {
	mu                 sync.Mutex
	encodeCalls        int
	setBitrateCalls    int
	setComplexityCalls int
	lastBitrate        int
	lastComplexity     int
	encodeErr          error
	encoded            []byte
}

func (m *mockStreamEncoder) Encode(pcm []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.encodeCalls++
	if m.encodeErr != nil {
		return nil, m.encodeErr
	}
	if m.encoded == nil {
		return []byte{1, 2, 3}, nil
	}
	return append([]byte(nil), m.encoded...), nil
}

func (m *mockStreamEncoder) SetBitrate(bitrate int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setBitrateCalls++
	m.lastBitrate = bitrate
	return nil
}

func (m *mockStreamEncoder) SetComplexity(complexity int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setComplexityCalls++
	m.lastComplexity = complexity
	return nil
}

func (m *mockStreamEncoder) Close() error { return nil }

func makeTestStoreWithSession(session uint32) *state.Store {
	st := state.NewStore()
	st.Self = &state.User{Session: session}
	return st
}

func newSenderWithMockEncoder(t *testing.T, c *mockSenderClient, cfg *StreamConfig) (*StreamSender, *mockStreamEncoder) {
	t.Helper()
	if cfg == nil {
		cfg = DefaultStreamConfig()
	}
	enc := &mockStreamEncoder{encoded: []byte{9, 8, 7, 6}}
	oldFactory := newStreamOpusEncoder
	newStreamOpusEncoder = func(cfg *StreamConfig) (streamEncoder, error) { return enc, nil }
	t.Cleanup(func() { newStreamOpusEncoder = oldFactory })

	sender, err := NewStreamSender(c, cfg)
	if err != nil {
		t.Fatalf("NewStreamSender() error = %v", err)
	}
	return sender, enc
}

func TestNewStreamSender(t *testing.T) {
	client := &mockSenderClient{}
	sender, _ := newSenderWithMockEncoder(t, client, DefaultStreamConfig())
	if sender == nil {
		t.Fatal("NewStreamSender() returned nil")
	}
	if sender.IsActive() {
		t.Fatal("expected IsActive=false before Start")
	}
}

func TestStreamSender_StartStop(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()
	cfg.BufferDepth = 1

	sender, _ := newSenderWithMockEncoder(t, client, cfg)
	if err := sender.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	sender.Stop()

	if sender.IsActive() {
		t.Fatal("expected IsActive=false after Stop")
	}
}

func TestStreamSender_SetConfigPropagatesBitrateAndComplexity(t *testing.T) {
	client := &mockSenderClient{}
	sender, enc := newSenderWithMockEncoder(t, client, DefaultStreamConfig())

	cfg := DefaultStreamConfig()
	cfg.Bitrate = 96000
	cfg.Complexity = 4
	if err := sender.SetConfig(cfg); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	enc.mu.Lock()
	defer enc.mu.Unlock()
	if enc.setBitrateCalls != 1 || enc.lastBitrate != 96000 {
		t.Fatalf("bitrate propagation mismatch: calls=%d bitrate=%d", enc.setBitrateCalls, enc.lastBitrate)
	}
	if enc.setComplexityCalls == 0 || enc.lastComplexity != 4 {
		t.Fatalf("complexity propagation mismatch: calls=%d complexity=%d", enc.setComplexityCalls, enc.lastComplexity)
	}
}

func TestStreamSender_DoSendAudioUsesSenderEncoderPath(t *testing.T) {
	client := &mockSenderClient{}
	sender, _ := newSenderWithMockEncoder(t, client, DefaultStreamConfig())

	sender.doSendAudio(make([]byte, 960*2))

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.sendAudioUDPCalled == 0 {
		t.Fatal("expected UDP send to be called")
	}
	if string(client.lastUDPPayload) != string([]byte{9, 8, 7, 6}) {
		t.Fatalf("expected encoded payload from sender encoder, got %v", client.lastUDPPayload)
	}
}

func TestStreamSender_MetadataSendsUserStateComment(t *testing.T) {
	client := &mockSenderClient{store: makeTestStoreWithSession(42)}
	sender, _ := newSenderWithMockEncoder(t, client, DefaultStreamConfig())

	if err := sender.SetMetadata(&StreamMetadata{Title: "Song", Artist: "Artist"}); err != nil {
		t.Fatalf("SetMetadata() error = %v", err)
	}
	time.Sleep(350 * time.Millisecond) // debounce flush

	client.mu.Lock()
	payload := append([]byte(nil), client.lastUserState...)
	calls := client.sendUserStateCalled
	client.mu.Unlock()

	if calls == 0 {
		t.Fatal("expected SendUserState to be called")
	}
	us, err := protocol.ParseUserState(payload)
	if err != nil {
		t.Fatalf("ParseUserState failed: %v", err)
	}
	if us.Session != 42 {
		t.Fatalf("expected session=42, got %d", us.Session)
	}
	if us.Comment == "" {
		t.Fatal("expected comment metadata payload to be set")
	}
}

func TestStreamSender_ReconnectBufferingOnUDPFallback(t *testing.T) {
	client := &mockSenderClient{udpErr: errors.New("udp down")}
	sender, _ := newSenderWithMockEncoder(t, client, DefaultStreamConfig())

	sender.doSendAudio(make([]byte, 960*2))

	if sender.reconn.BufferedCount() == 0 {
		t.Fatal("expected frame buffered during UDP failure")
	}
	client.mu.Lock()
	tcpCalls := client.sendAudioCalled
	client.mu.Unlock()
	if tcpCalls == 0 {
		t.Fatal("expected TCP fallback send on UDP failure")
	}

	client.mu.Lock()
	client.udpErr = nil
	client.mu.Unlock()
	sender.doSendAudio(make([]byte, 960*2))

	if sender.reconn.BufferedCount() != 0 {
		t.Fatalf("expected buffered frames to flush on UDP recovery, got %d", sender.reconn.BufferedCount())
	}
}
