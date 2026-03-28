package stream

import (
	"sync"
	"time"
)

// ReconnectCallback is called when the reconnect manager decides to reconnect.
// It should close the old connection and establish a new one, then return nil
// on success or an error on failure.
type ReconnectCallback func() error

// ReconnectManager handles reconnection logic and audio buffering during outages.
type ReconnectManager struct {
	mu     sync.Mutex
	config *StreamConfig
	closed bool

	// Reconnection state
	attempts       int
	nextAttemptAt  time.Time
	reconnectTimer *time.Timer
	lastErr        error

	// Audio buffer during reconnection
	audioBuffer     [][]byte
	audioBufferHead int
	onReconnecting  func(attempt int, nextDelay time.Duration)
	onReconnected   func()
	onBufferFull    func(dropped int)
}

// NewReconnectManager creates a ReconnectManager with the given config.
func NewReconnectManager(cfg *StreamConfig) *ReconnectManager {
	return &ReconnectManager{
		config:      cfg,
		audioBuffer: make([][]byte, 0, cfg.ReconnectBufferSize),
	}
}

// SetReconnectingHandler sets the callback for reconnection attempts.
func (r *ReconnectManager) SetReconnectingHandler(h func(attempt int, nextDelay time.Duration)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onReconnecting = h
}

// SetReconnectedHandler sets the callback for successful reconnection.
func (r *ReconnectManager) SetReconnectedHandler(h func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onReconnected = h
}

// SetBufferFullHandler sets the callback when buffer is full and old frames are dropped.
func (r *ReconnectManager) SetBufferFullHandler(h func(dropped int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onBufferFull = h
}

// OnDisconnect is called when the connection is lost.
// It starts the reconnection timer if enabled.
func (r *ReconnectManager) OnDisconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed || !r.config.Reconnect {
		return
	}

	r.attempts = 1
	r.lastErr = nil

	delay := r.config.InitialDelay
	r.nextAttemptAt = time.Now().Add(delay)

	if r.reconnectTimer != nil {
		r.reconnectTimer.Stop()
	}
	r.reconnectTimer = time.AfterFunc(delay, func() {
		r.mu.Lock()
		attempt := r.attempts
		delay := time.Until(r.nextAttemptAt)
		r.mu.Unlock()
		if r.onReconnecting != nil {
			r.onReconnecting(attempt, delay)
		}
	})
}

// BufferFrame adds a frame to the reconnection buffer.
// If the buffer is full, the oldest frame is dropped.
func (r *ReconnectManager) BufferFrame(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	if len(r.audioBuffer) >= cap(r.audioBuffer) {
		// Drop oldest frame
		dropped := 1
		if len(r.audioBuffer) > 0 {
			r.audioBuffer = r.audioBuffer[1:]
		}
		if r.onBufferFull != nil {
			r.onBufferFull(dropped)
		}
	}

	// Copy the frame since the caller's slice may be reused
	frameCopy := make([]byte, len(frame))
	copy(frameCopy, frame)
	r.audioBuffer = append(r.audioBuffer, frameCopy)
}

// PopBuffered returns buffered frames in FIFO order.
// It returns nil if the buffer is empty.
func (r *ReconnectManager) PopBuffered() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.audioBuffer) == 0 {
		return nil
	}

	frames := r.audioBuffer
	r.audioBuffer = make([][]byte, 0, r.config.ReconnectBufferSize)
	return frames
}

// BufferedCount returns the number of frames currently buffered.
func (r *ReconnectManager) BufferedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.audioBuffer)
}

// OnReconnectSuccess resets reconnection state and clears the buffer.
func (r *ReconnectManager) OnReconnectSuccess() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attempts = 0
	r.lastErr = nil
	if r.reconnectTimer != nil {
		r.reconnectTimer.Stop()
		r.reconnectTimer = nil
	}
	r.audioBuffer = r.audioBuffer[:0]

	if r.onReconnected != nil {
		r.onReconnected()
	}
}

// OnReconnectFailure is called when a reconnection attempt fails.
// It schedules the next attempt using exponential backoff.
func (r *ReconnectManager) OnReconnectFailure(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	r.lastErr = err
	r.attempts++

	if r.attempts > r.config.MaxAttempts {
		// Max attempts reached
		return
	}

	// Exponential backoff: delay *= backoffMultiplier
	delay := r.config.InitialDelay
	for i := 1; i < r.attempts; i++ {
		delay = time.Duration(float64(delay) * r.config.BackoffMultiplier)
		if delay > r.config.MaxDelay {
			delay = r.config.MaxDelay
			break
		}
	}

	r.nextAttemptAt = time.Now().Add(delay)

	if r.reconnectTimer != nil {
		r.reconnectTimer.Stop()
	}
	r.reconnectTimer = time.AfterFunc(delay, func() {
		r.mu.Lock()
		attempt := r.attempts
		delay := time.Until(r.nextAttemptAt)
		r.mu.Unlock()
		if r.onReconnecting != nil {
			r.onReconnecting(attempt, delay)
		}
	})
}

// MaxAttemptsReached returns true if reconnection has failed maxAttempts times.
func (r *ReconnectManager) MaxAttemptsReached() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempts > r.config.MaxAttempts
}

// Close stops the reconnect manager.
func (r *ReconnectManager) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.reconnectTimer != nil {
		r.reconnectTimer.Stop()
	}
	r.audioBuffer = nil
}
