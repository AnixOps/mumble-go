package stream

import (
	"sync"
	"testing"
	"time"
)

func TestReconnectManager_Basic(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond
	cfg.MaxDelay = 100 * time.Millisecond
	cfg.BackoffMultiplier = 2.0

	m := NewReconnectManager(cfg)

	if m.MaxAttemptsReached() {
		t.Fatal("expected MaxAttemptsReached=false initially")
	}
}

func TestReconnectManager_OnDisconnect(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	m := NewReconnectManager(cfg)

	// Call OnDisconnect to start reconnection
	m.OnDisconnect()

	// Small delay to let timer fire
	time.Sleep(15 * time.Millisecond)
}

func TestReconnectManager_BufferFrame(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.ReconnectBufferSize = 3

	m := NewReconnectManager(cfg)

	frame := []byte{1, 2, 3, 4, 5}
	m.BufferFrame(frame)

	if m.BufferedCount() != 1 {
		t.Fatalf("expected 1 buffered frame, got %d", m.BufferedCount())
	}
}

func TestReconnectManager_BufferFrameOverflow(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.ReconnectBufferSize = 2

	m := NewReconnectManager(cfg)

	// Add more frames than buffer can hold
	m.BufferFrame([]byte{1})
	m.BufferFrame([]byte{2})
	m.BufferFrame([]byte{3}) // Should drop first frame

	if m.BufferedCount() != 2 {
		t.Fatalf("expected 2 buffered frames (capacity), got %d", m.BufferedCount())
	}
}

func TestReconnectManager_PopBuffered(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.ReconnectBufferSize = 10

	m := NewReconnectManager(cfg)

	m.BufferFrame([]byte{1, 2, 3})
	m.BufferFrame([]byte{4, 5, 6})

	frames := m.PopBuffered()
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}

	if m.BufferedCount() != 0 {
		t.Fatalf("expected 0 after pop, got %d", m.BufferedCount())
	}
}

func TestReconnectManager_OnReconnectSuccess(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	m := NewReconnectManager(cfg)

	m.OnDisconnect()
	m.OnReconnectSuccess()

	if m.BufferedCount() != 0 {
		t.Fatalf("expected 0 buffered after success, got %d", m.BufferedCount())
	}
}

func TestReconnectManager_OnReconnectFailure(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond
	cfg.MaxDelay = 100 * time.Millisecond
	cfg.BackoffMultiplier = 2.0

	m := NewReconnectManager(cfg)

	m.OnDisconnect()
	m.OnReconnectFailure(nil)

	// Should not be at max attempts yet
	if m.MaxAttemptsReached() {
		t.Fatal("expected MaxAttemptsReached=false after 1 failure")
	}
}

func TestReconnectManager_MaxAttemptsReached(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 2
	cfg.InitialDelay = 10 * time.Millisecond

	m := NewReconnectManager(cfg)

	m.OnDisconnect()
	m.OnReconnectFailure(nil)
	m.OnReconnectFailure(nil)

	if !m.MaxAttemptsReached() {
		t.Fatal("expected MaxAttemptsReached=true after 2 failures")
	}
}

func TestReconnectManager_Handlers(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 10 * time.Millisecond

	m := NewReconnectManager(cfg)

	var mu sync.Mutex
	reconnectingCalled := false
	reconnectedCalled := false

	m.SetReconnectingHandler(func(attempt int, nextDelay time.Duration) {
		mu.Lock()
		reconnectingCalled = true
		mu.Unlock()
	})

	m.SetReconnectedHandler(func() {
		mu.Lock()
		reconnectedCalled = true
		mu.Unlock()
	})

	m.OnDisconnect()
	// Give timer time to fire - timer fires after InitialDelay
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	if !reconnectingCalled {
		mu.Unlock()
		t.Fatal("expected reconnecting handler to be called")
	}
	mu.Unlock()

	// Simulate successful reconnection
	m.OnReconnectSuccess()
	mu.Lock()
	if !reconnectedCalled {
		mu.Unlock()
		t.Fatal("expected reconnected handler to be called")
	}
	mu.Unlock()
}

func TestReconnectManager_BufferSize(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.ReconnectBufferSize = 50

	m := NewReconnectManager(cfg)

	// Fill buffer
	for i := 0; i < 50; i++ {
		m.BufferFrame([]byte{byte(i)})
	}

	if m.BufferedCount() != 50 {
		t.Fatalf("expected BufferedCount=50, got %d", m.BufferedCount())
	}
}

func TestReconnectManager_Close(t *testing.T) {
	cfg := DefaultStreamConfig()
	cfg.Reconnect = true
	cfg.MaxAttempts = 3

	m := NewReconnectManager(cfg)

	m.BufferFrame([]byte{1, 2, 3})
	m.Close()

	// After close, buffering should be no-op
	m.BufferFrame([]byte{4, 5, 6})
	if m.BufferedCount() != 0 {
		t.Fatalf("expected 0 after close, got %d", m.BufferedCount())
	}
}
