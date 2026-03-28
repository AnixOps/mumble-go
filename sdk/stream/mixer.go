package stream

import (
	"context"
	"sync"
)

// mixedSource represents a single input to the mixer.
type mixedSource struct {
	id    string
	src   AudioSource
	gain  float64
	gainMu sync.RWMutex
}

// AudioMixer mixes multiple AudioSources into a single PCM stream.
// Each source can have an independent gain level.
type AudioMixer struct {
	sources   map[string]*mixedSource
	mu        sync.RWMutex
	frameSize int // samples per frame (960 for 20ms at 48kHz)
	closed    bool
}

// AudioSource is the interface for audio sources (compatible with sdk.AudioSource).
type AudioSource interface {
	ReadPCM(ctx context.Context, dst []byte) (int, error)
}

// NewAudioMixer creates a mixer that outputs frames of the given size in samples.
func NewAudioMixer(frameSize int) *AudioMixer {
	return &AudioMixer{
		sources:   make(map[string]*mixedSource),
		frameSize: frameSize,
	}
}

// AddSource adds an audio source with the given gain (1.0 = unity).
func (m *AudioMixer) AddSource(id string, src AudioSource, gain float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[id] = &mixedSource{id: id, src: src, gain: gain}
}

// RemoveSource removes and stops a source.
func (m *AudioMixer) RemoveSource(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sources, id)
}

// SetGain updates the gain for a source.
func (m *AudioMixer) SetGain(id string, gain float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sources[id]; ok {
		s.gainMu.Lock()
		s.gain = gain
		s.gainMu.Unlock()
	}
}

// ReadMix reads one mixed frame from all sources into dst.
// It blocks until at least one source provides data or an error occurs.
// If no source provides data, it returns 0, nil.
func (m *AudioMixer) ReadMix(ctx context.Context, dst []byte) (int, error) {
	m.mu.RLock()
	sources := make([]*mixedSource, 0, len(m.sources))
	for _, s := range m.sources {
		sources = append(sources, s)
	}
	m.mu.RUnlock()

	if len(sources) == 0 {
		// No sources — return silence
		clearPCM(dst)
		return len(dst), nil
	}

	frameBytes := m.frameSize * 2 // 16-bit samples
	accum := make([]float64, m.frameSize)

	for _, s := range sources {
		buf := make([]byte, frameBytes)
		n, err := s.src.ReadPCM(ctx, buf)
		if err != nil || n == 0 {
			continue
		}

		s.gainMu.RLock()
		gain := s.gain
		s.gainMu.RUnlock()

		samples := n / 2
		for i := 0; i < samples && i < m.frameSize; i++ {
			sample := int16(buf[i*2]) | (int16(buf[i*2+1]) << 8)
			accum[i] += float64(sample) * gain
		}
	}

	// Clamp and convert back to int16
	var maxVal float64 = 32767
	var minVal float64 = -32768
	for i := 0; i < m.frameSize; i++ {
		v := accum[i]
		if v > maxVal {
			v = maxVal
		} else if v < minVal {
			v = minVal
		}
		sample := int16(v)
		dst[i*2] = byte(sample)
		dst[i*2+1] = byte(sample >> 8)
	}

	return frameBytes, nil
}

// Close stops all sources and the mixer.
func (m *AudioMixer) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for _, s := range m.sources {
		if closer, ok := s.src.(interface{ Close() }); ok {
			closer.Close()
		}
	}
	m.sources = nil
}

func clearPCM(dst []byte) {
	for i := range dst {
		dst[i] = 0
	}
}
