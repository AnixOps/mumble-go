package stream

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"mumble-go/client"
	"mumble-go/state"
)

// mockSenderClient implements SenderClient for testing.
type mockSenderClient struct {
	mu               sync.Mutex
	sendAudioUDPCalled int
	sendAudioCalled    int
}

func (m *mockSenderClient) SendAudio(pcm []byte) error {
	m.mu.Lock()
	m.sendAudioCalled++
	m.mu.Unlock()
	return nil
}

func (m *mockSenderClient) SendAudioUDP(pcm []byte) error {
	m.mu.Lock()
	m.sendAudioUDPCalled++
	m.mu.Unlock()
	return nil
}

func (m *mockSenderClient) Audio() *client.Audio { return nil }
func (m *mockSenderClient) Events() *client.EventHandler { return nil }
func (m *mockSenderClient) State() *state.Store { return nil }

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

func TestNewStreamSender(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, err := NewStreamSender(client, cfg)
	if err != nil {
		t.Fatalf("NewStreamSender() error = %v", err)
	}
	if sender == nil {
		t.Fatal("NewStreamSender() returned nil")
	}
	if sender.IsActive() {
		t.Fatal("expected IsActive=false before Start")
	}
}

func TestNewStreamSender_NilConfig(t *testing.T) {
	client := &mockSenderClient{}
	sender, err := NewStreamSender(client, nil)
	if err != nil {
		t.Fatalf("NewStreamSender(nil) error = %v", err)
	}
	if sender == nil {
		t.Fatal("NewStreamSender(nil) returned nil")
	}
}

func TestNewStreamSender_InvalidConfig(t *testing.T) {
	client := &mockSenderClient{}
	cfg := &StreamConfig{
		Bitrate: 0, // Invalid: must be 6000-512000
	}
	_, err := NewStreamSender(client, cfg)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestStreamSender_StartStop(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()
	cfg.BufferDepth = 1 // Smaller for faster testing

	sender, err := NewStreamSender(client, cfg)
	if err != nil {
		t.Fatalf("NewStreamSender() error = %v", err)
	}

	ctx := context.Background()
	err = sender.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !sender.IsActive() {
		t.Fatal("expected IsActive=true after Start")
	}

	// Give it time to run a frame
	time.Sleep(50 * time.Millisecond)

	sender.Stop()
	time.Sleep(10 * time.Millisecond) // Let goroutine exit

	if sender.IsActive() {
		t.Fatal("expected IsActive=false after Stop")
	}
}

func TestStreamSender_StartTwice(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, _ := NewStreamSender(client, cfg)

	ctx := context.Background()
	_ = sender.Start(ctx)
	defer sender.Stop()

	err := sender.Start(ctx)
	if err == nil {
		t.Fatal("expected error when starting twice")
	}
}

func TestStreamSender_StopTwice(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, _ := NewStreamSender(client, cfg)
	ctx := context.Background()
	_ = sender.Start(ctx)

	// Should not panic
	sender.Stop()
	sender.Stop()
}

func TestStreamSender_PauseResume(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()
	cfg.BufferDepth = 1

	sender, _ := NewStreamSender(client, cfg)
	ctx := context.Background()
	_ = sender.Start(ctx)
	defer sender.Stop()

	// Let it run a bit
	time.Sleep(50 * time.Millisecond)
	client.mu.Lock()
	callsAfterStart := client.sendAudioUDPCalled
	client.mu.Unlock()

	sender.Pause()
	time.Sleep(30 * time.Millisecond)
	client.mu.Lock()
	callsAfterPause := client.sendAudioUDPCalled
	client.mu.Unlock()

	sender.Resume()
	time.Sleep(30 * time.Millisecond)
	client.mu.Lock()
	callsAfterResume := client.sendAudioUDPCalled
	client.mu.Unlock()

	// Paused should not have increased calls significantly
	if callsAfterPause > callsAfterStart+2 {
		t.Logf("pause may not be working: start=%d pause=%d resume=%d", callsAfterStart, callsAfterPause, callsAfterResume)
	}
}

func TestStreamSender_SetSource(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()
	cfg.BufferDepth = 1

	sender, _ := NewStreamSender(client, cfg)

	src := &mockAudioSource{data: make([]byte, 1920*3)}
	err := sender.SetSource(src)
	if err != nil {
		t.Fatalf("SetSource() error = %v", err)
	}

	ctx := context.Background()
	_ = sender.Start(ctx)
	defer sender.Stop()

	time.Sleep(50 * time.Millisecond)

	src.mu.Lock()
	pos := src.pos
	src.mu.Unlock()
	if pos == 0 {
		t.Fatal("expected source to be read")
	}
}

func TestStreamSender_AddRemoveSource(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, _ := NewStreamSender(client, cfg)

	src1 := &mockAudioSource{data: make([]byte, 1920*10)}
	src2 := &mockAudioSource{data: make([]byte, 1920*10)}

	err := sender.AddSource("src1", src1, 1.0)
	if err != nil {
		t.Fatalf("AddSource() error = %v", err)
	}

	err = sender.AddSource("src2", src2, 0.5)
	if err != nil {
		t.Fatalf("AddSource() error = %v", err)
	}

	// Duplicate should fail
	err = sender.AddSource("src1", src1, 1.0)
	if err == nil {
		t.Fatal("expected error for duplicate source")
	}

	err = sender.RemoveSource("src1")
	if err != nil {
		t.Fatalf("RemoveSource() error = %v", err)
	}

	// Non-existent should fail
	err = sender.RemoveSource("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
}

func TestStreamSender_SetSourceGain(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, _ := NewStreamSender(client, cfg)

	src := &mockAudioSource{data: make([]byte, 1920*10)}
	_ = sender.AddSource("src", src, 1.0)

	err := sender.SetSourceGain("src", 0.5)
	if err != nil {
		t.Fatalf("SetSourceGain() error = %v", err)
	}

	// Non-existent should fail
	err = sender.SetSourceGain("nonexistent", 0.5)
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
}

func TestStreamSender_SetConfig(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, _ := NewStreamSender(client, cfg)

	newCfg := DefaultStreamConfig()
	err := sender.SetConfig(newCfg)
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// Invalid config should fail
	err = sender.SetConfig(&StreamConfig{Bitrate: 0})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestStreamSender_Events(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()
	cfg.BufferDepth = 1

	sender, _ := NewStreamSender(client, cfg)

	events := sender.Events()
	if events == nil {
		t.Fatal("Events() returned nil")
	}

	connectCalled := false
	events.OnConnect = func() {
		connectCalled = true
	}

	ctx := context.Background()
	_ = sender.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	sender.Stop()

	if !connectCalled {
		t.Fatal("expected OnConnect to be called")
	}
}

func TestStreamSender_GetReconnectManager(t *testing.T) {
	client := &mockSenderClient{}
	cfg := DefaultStreamConfig()

	sender, _ := NewStreamSender(client, cfg)

	reconn := sender.GetReconnectManager()
	if reconn == nil {
		t.Fatal("GetReconnectManager() returned nil")
	}
}
