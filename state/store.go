package state

import "sync"

// Store keeps the latest server snapshot.
type Store struct {
	mu       sync.RWMutex
	Self     *User
	Users    map[uint32]User
	Channels map[uint32]Channel
	Ready    bool
}

func NewStore() *Store {
	return &Store{
		Users:    make(map[uint32]User),
		Channels: make(map[uint32]Channel),
	}
}

func (s *Store) SnapshotUsers() map[uint32]User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[uint32]User, len(s.Users))
	for k, v := range s.Users {
		out[k] = v
	}
	return out
}

func (s *Store) SnapshotChannels() map[uint32]Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[uint32]Channel, len(s.Channels))
	for k, v := range s.Channels {
		out[k] = v
	}
	return out
}

func (s *Store) SelfSession() uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.Self == nil {
		return 0
	}
	return s.Self.Session
}

func (s *Store) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Ready
}
