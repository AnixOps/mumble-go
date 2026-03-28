package stream

import (
	"encoding/json"
	"sync"
	"time"
)

// StreamMetadata holds track information displayed to other users.
type StreamMetadata struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	CoverURL string `json:"cover_url,omitempty"`
}

// MetadataUpdater sends metadata changes to the Mumble server via UserState.
type MetadataUpdater struct {
	client   MetadataClient
	debounce time.Duration
	mu       sync.Mutex
	current  *StreamMetadata
	pending  *StreamMetadata
	timer    *time.Timer
	closeCh  chan struct{}
}

// MetadataClient abstracts the minimal client methods we need for metadata.
type MetadataClient interface {
	SendUserState(marshal func() ([]byte, error)) error
}

// NewMetadataUpdater creates a MetadataUpdater that debounces metadata changes.
func NewMetadataUpdater(client MetadataClient) *MetadataUpdater {
	return &MetadataUpdater{
		client:   client,
		debounce: 300 * time.Millisecond,
		closeCh:  make(chan struct{}),
	}
}

// Set queues a metadata update with debouncing.
func (m *MetadataUpdater) Set(meta *StreamMetadata) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pending = meta

	if m.timer == nil {
		m.timer = time.AfterFunc(m.debounce, m.flush)
	} else {
		m.timer.Reset(m.debounce)
	}
}

// flush sends the current pending metadata immediately.
func (m *MetadataUpdater) flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pending == nil {
		return
	}

	// Don't resend if unchanged
	if m.current != nil && metadataEqual(m.current, m.pending) {
		m.pending = nil
		return
	}

	data, err := json.Marshal(m.pending)
	if err != nil {
		return
	}

	comment := string(data)
	m.client.SendUserState(func() ([]byte, error) {
		return []byte(comment), nil
	})

	m.current = m.pending
	m.pending = nil
}

// Close stops the debounce timer.
func (m *MetadataUpdater) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.timer != nil {
		m.timer.Stop()
	}
	close(m.closeCh)
}

func metadataEqual(a, b *StreamMetadata) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Title == b.Title && a.Artist == b.Artist && a.Album == b.Album && a.CoverURL == b.CoverURL
}
