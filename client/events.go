package client

import (
	"sync"
)

// EventHandler handles client events.
type EventHandler struct {
	mu sync.RWMutex

	// User events
	onUserJoined    func(session uint32)
	onUserLeft      func(session uint32)
	onUserState     func(session uint32)

	// Channel events
	onChannelAdded func(channelID uint32)
	onChannelRemoved func(channelID uint32)

	// Audio events
	onAudio         func(session uint32, seq uint64, pcm []byte)

	// Connection events
	onConnect       func()
	onDisconnect    func()

	// Message events
	onTextMessage   func(actor uint32, message string)
}

// NewEventHandler creates a new event handler.
func NewEventHandler() *EventHandler {
	return &EventHandler{}
}

// OnUserJoined registers a callback for when a user joins.
func (h *EventHandler) OnUserJoined(cb func(session uint32)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onUserJoined = cb
}

// OnUserLeft registers a callback for when a user leaves.
func (h *EventHandler) OnUserLeft(cb func(session uint32)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onUserLeft = cb
}

// OnUserState registers a callback for user state changes.
func (h *EventHandler) OnUserState(cb func(session uint32)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onUserState = cb
}

// OnChannelAdded registers a callback for when a channel is added.
func (h *EventHandler) OnChannelAdded(cb func(channelID uint32)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onChannelAdded = cb
}

// OnChannelRemoved registers a callback for when a channel is removed.
func (h *EventHandler) OnChannelRemoved(cb func(channelID uint32)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onChannelRemoved = cb
}

// OnAudio registers a callback for received audio.
func (h *EventHandler) OnAudio(cb func(session uint32, seq uint64, pcm []byte)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onAudio = cb
}

// OnConnect registers a callback for when the client connects.
func (h *EventHandler) OnConnect(cb func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onConnect = cb
}

// OnDisconnect registers a callback for when the client disconnects.
func (h *EventHandler) OnDisconnect(cb func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onDisconnect = cb
}

// OnTextMessage registers a callback for text messages.
func (h *EventHandler) OnTextMessage(cb func(actor uint32, message string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onTextMessage = cb
}

// emitUserJoined calls the user joined callback.
func (h *EventHandler) emitUserJoined(session uint32) {
	h.mu.RLock()
	cb := h.onUserJoined
	h.mu.RUnlock()
	if cb != nil {
		cb(session)
	}
}

// emitUserLeft calls the user left callback.
func (h *EventHandler) emitUserLeft(session uint32) {
	h.mu.RLock()
	cb := h.onUserLeft
	h.mu.RUnlock()
	if cb != nil {
		cb(session)
	}
}

// emitUserState calls the user state callback.
func (h *EventHandler) emitUserState(session uint32) {
	h.mu.RLock()
	cb := h.onUserState
	h.mu.RUnlock()
	if cb != nil {
		cb(session)
	}
}

// emitChannelAdded calls the channel added callback.
func (h *EventHandler) emitChannelAdded(channelID uint32) {
	h.mu.RLock()
	cb := h.onChannelAdded
	h.mu.RUnlock()
	if cb != nil {
		cb(channelID)
	}
}

// emitChannelRemoved calls the channel removed callback.
func (h *EventHandler) emitChannelRemoved(channelID uint32) {
	h.mu.RLock()
	cb := h.onChannelRemoved
	h.mu.RUnlock()
	if cb != nil {
		cb(channelID)
	}
}

// emitAudio calls the audio callback.
func (h *EventHandler) emitAudio(session uint32, seq uint64, pcm []byte) {
	h.mu.RLock()
	cb := h.onAudio
	h.mu.RUnlock()
	if cb != nil {
		cb(session, seq, pcm)
	}
}

// emitConnect calls the connect callback.
func (h *EventHandler) emitConnect() {
	h.mu.RLock()
	cb := h.onConnect
	h.mu.RUnlock()
	if cb != nil {
		cb()
	}
}

// emitDisconnect calls the disconnect callback.
func (h *EventHandler) emitDisconnect() {
	h.mu.RLock()
	cb := h.onDisconnect
	h.mu.RUnlock()
	if cb != nil {
		cb()
	}
}

// emitTextMessage calls the text message callback.
func (h *EventHandler) emitTextMessage(actor uint32, message string) {
	h.mu.RLock()
	cb := h.onTextMessage
	h.mu.RUnlock()
	if cb != nil {
		cb(actor, message)
	}
}
