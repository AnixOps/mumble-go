package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mumble-go/audio"
	"mumble-go/client"
	"mumble-go/state"
)

// SenderClient is the interface for the client used by StreamSender.
type SenderClient interface {
	SendAudio(pcm []byte) error
	SendAudioUDP(pcm []byte) error
	Audio() *client.Audio
	Events() *client.EventHandler
	State() *state.Store
}

// StreamSender is a non-blocking audio streaming sender with jitter buffering,
// voice activity detection, multi-source mixing, and reconnection support.
type StreamSender struct {
	// Immutable after construction
	client SenderClient
	config *StreamConfig
	events StreamEvents

	// Runtime state (protected by mu)
	mu      sync.Mutex
	closed  bool
	active  bool
	paused  bool
	sources map[string]AudioSource // sourceID -> AudioSource
	mixer   *AudioMixer
	jitter  *JitterBuffer
	vad     *VAD
	reconn  *ReconnectManager
	meta    *MetadataUpdater
	metaCl  *metadataClient

	// Channels for lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// metadataClient wraps a SenderClient to implement MetadataClient.
type metadataClient struct {
	sender SenderClient
}

func (m *metadataClient) SendUserState(marshal func() ([]byte, error)) error {
	data, err := marshal()
	if err != nil {
		return err
	}
	_ = data
	return nil
}

// NewStreamSender creates a new StreamSender backed by the given client.
// The client must already be connected before calling Start().
func NewStreamSender(client SenderClient, cfg *StreamConfig) (*StreamSender, error) {
	if cfg == nil {
		cfg = DefaultStreamConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("stream sender: invalid config: %w", err)
	}

	s := &StreamSender{
		client:   client,
		config:   cfg,
		sources:  make(map[string]AudioSource),
		mixer:    NewAudioMixer(audio.FrameSize),
		jitter:   NewJitterBuffer(cfg.BufferDepth, audio.FrameDurationMs*time.Millisecond),
		reconn:   NewReconnectManager(cfg),
		stopCh:   make(chan struct{}),
	}

	// VAD
	if cfg.VADEnabled {
		s.vad = NewVAD(cfg.EnergyThreshold, func(speaking bool) {
			ev := s.getEvents()
			if ev != nil && ev.OnVADChange != nil {
				ev.OnVADChange(speaking)
			}
		})
	}

	// Metadata updater
	s.metaCl = &metadataClient{sender: client}
	s.meta = NewMetadataUpdater(s.metaCl)

	// Wire up reconnect manager callbacks
	s.reconn.SetReconnectingHandler(func(attempt int, nextDelay time.Duration) {
		ev := s.getEvents()
		if ev != nil && ev.OnReconnecting != nil {
			ev.OnReconnecting(attempt, nextDelay)
		}
	})
	s.reconn.SetReconnectedHandler(func() {
		ev := s.getEvents()
		if ev != nil && ev.OnConnect != nil {
			ev.OnConnect()
		}
	})

	return s, nil
}

// getEvents returns a copy of the current events struct (thread-safe).
func (s *StreamSender) getEvents() *StreamEvents {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &s.events
}

// Start begins the streaming goroutine. It is non-blocking.
// The provided context controls the overall streaming lifetime.
func (s *StreamSender) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stream sender: already closed")
	}
	if s.active {
		return fmt.Errorf("stream sender: already active")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.active = true
	s.paused = false

	s.wg.Add(1)
	go s.run()

	ev := &s.events
	if ev != nil && ev.OnConnect != nil {
		ev.OnConnect()
	}

	return nil
}

// Stop stops the streaming goroutine and releases resources.
// It is safe to call multiple times.
func (s *StreamSender) Stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.active = false
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()

	s.jitter.Close()
	s.mixer.Close()
	s.reconn.Close()
	s.meta.Close()

	ev := &s.events
	if ev != nil && ev.OnDisconnect != nil {
		ev.OnDisconnect(nil)
	}
}

// Pause temporarily stops sending audio (but keeps the stream alive).
func (s *StreamSender) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
}

// Resume resumes audio transmission after a Pause.
func (s *StreamSender) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = false
}

// IsActive returns true if the stream is currently running.
func (s *StreamSender) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active && !s.closed
}

// SetSource sets a single audio source (replaces any existing sources).
// This is for single-source mode; for multi-source mode use AddSource.
func (s *StreamSender) SetSource(source AudioSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing sources
	for id := range s.sources {
		delete(s.sources, id)
	}
	s.mixer = NewAudioMixer(audio.FrameSize)

	if source != nil {
		s.sources["__single__"] = source
		s.mixer.AddSource("__single__", source, 1.0)
	}
	return nil
}

// AddSource adds an audio source with the given gain to the mix.
// Each source must have a unique id.
func (s *StreamSender) AddSource(id string, source AudioSource, gain float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sources[id]; exists {
		return fmt.Errorf("stream sender: source %q already exists", id)
	}

	s.sources[id] = source
	s.mixer.AddSource(id, source, gain)
	return nil
}

// RemoveSource removes an audio source from the mix.
func (s *StreamSender) RemoveSource(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sources[id]; !exists {
		return fmt.Errorf("stream sender: source %q not found", id)
	}

	delete(s.sources, id)
	s.mixer.RemoveSource(id)
	return nil
}

// SetSourceGain updates the gain for a source.
func (s *StreamSender) SetSourceGain(id string, gain float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sources[id]; !exists {
		return fmt.Errorf("stream sender: source %q not found", id)
	}

	s.mixer.SetGain(id, gain)
	return nil
}

// SetMetadata updates the stream metadata (title, artist, etc.).
func (s *StreamSender) SetMetadata(meta *StreamMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.meta.Set(meta)

	ev := &s.events
	if ev != nil && ev.OnMetadataSet != nil {
		ev.OnMetadataSet(meta)
	}
	return nil
}

// SetConfig updates the stream configuration at runtime.
func (s *StreamSender) SetConfig(cfg *StreamConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
	return nil
}

// Events returns the events struct for setting callbacks.
func (s *StreamSender) Events() *StreamEvents {
	return &s.events
}

// run is the main streaming loop.
func (s *StreamSender) run() {
	defer s.wg.Done()

	frameBytes := audio.FrameSize * 2 // 960 samples * 2 bytes
	silence := make([]byte, frameBytes)
	frameDuration := time.Duration(audio.FrameDurationMs) * time.Millisecond
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	sequence := uint64(0)

	for {
		select {
		case <-s.stopCh:
			return
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			sequence = s.sendFrame(silence, frameBytes, sequence)
		}
	}
}

// sendFrame reads from the mixer, applies VAD, and sends audio.
// Returns the updated sequence number.
func (s *StreamSender) sendFrame(silence []byte, frameBytes int, seq uint64) uint64 {
	s.mu.Lock()
	paused := s.paused
	s.mu.Unlock()

	if paused {
		return seq
	}

	// Read mixed PCM from all sources
	pcm := make([]byte, frameBytes)
	n, err := s.mixer.ReadMix(s.ctx, pcm)
	if err != nil || n == 0 {
		// Source error or EOF
		if err != nil && err.Error() != "EOF" {
			ev := s.getEvents()
			if ev != nil && ev.OnError != nil {
				ev.OnError(err)
			}
		}
		// Try to get frame from jitter buffer if available
		if s.jitter != nil {
			jitterPcm, ok := s.jitter.Pop(time.Now())
			if ok && len(jitterPcm) == frameBytes {
				s.doSendAudio(jitterPcm)
				return seq + 1
			}
		}
		// Send silence to keep stream alive
		s.doSendAudio(silence)
		return seq + 1
	}

	// VAD check
	s.mu.Lock()
	vad := s.vad
	skipSilent := s.config.SkipSilentFrames
	s.mu.Unlock()
	isSpeaking := true
	if vad != nil {
		isSpeaking = vad.Process(pcm) // triggers callback if state changed
	}

	// Skip sending if VAD is enabled, SkipSilentFrames is true, and not speaking
	if !isSpeaking && skipSilent {
		// Don't send anything to save bandwidth - stream will timeout if no audio received
		return seq
	}

	// Push to jitter buffer for smoothing
	// Deadline = now + buffer depth * frame duration (target playback time)
	if s.jitter != nil {
		deadline := time.Now().Add(time.Duration(s.config.BufferDepth) * time.Duration(audio.FrameDurationMs) * time.Millisecond)
		s.jitter.Push(pcm, deadline)
		// Pop the frame - jitter buffer handles timing
		jitterPcm, ok := s.jitter.Pop(time.Now())
		if ok && len(jitterPcm) == frameBytes {
			s.doSendAudio(jitterPcm)
			return seq + 1
		}
		// If jitter buffer not ready, fall through to direct send
	}

	s.doSendAudio(pcm)
	return seq + 1
}

// doSendAudio sends PCM audio via the client.
func (s *StreamSender) doSendAudio(pcm []byte) {
	// Try UDP first for lower latency, fall back to TCP
	if err := s.client.SendAudioUDP(pcm); err != nil {
		// If UDP fails, try TCP
		if tcpErr := s.client.SendAudio(pcm); tcpErr != nil {
			ev := s.getEvents()
			if ev != nil && ev.OnError != nil {
				ev.OnError(fmt.Errorf("send audio (UDP:%v, TCP:%v)", err, tcpErr))
			}
		}
	}
}

// GetReconnectManager returns the reconnect manager for configuration.
// Note: The caller is responsible for implementing the actual reconnection
// (closing the old connection and establishing a new one).
// The ReconnectManager only handles buffering during reconnection.
func (s *StreamSender) GetReconnectManager() *ReconnectManager {
	return s.reconn
}
