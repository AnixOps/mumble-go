package state

import (
	"mumble-go/protocol"
)

// MarkReady flips the store into ready state.
func (s *Store) MarkReady() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Ready = true
}

// UpsertUserFromProto parses a UserState protobuf message and upserts it.
func (s *Store) UpsertUserFromProto(cs *protocol.UserState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u := s.Users[cs.Session]
	u.Session = cs.Session
	u.UserID = cs.UserID
	if cs.Name != "" {
		u.Name = cs.Name
	}
	if cs.HasChannelID {
		u.ChannelID = cs.ChannelID
		u.HasChannelID = true
	}
	u.Muted = cs.Mute
	u.Deafened = cs.Deaf
	u.SelfMute = cs.SelfMute
	u.SelfDeaf = cs.SelfDeaf
	u.Suppress = cs.Suppress
	u.Recording = cs.Recording
	if len(cs.Texture) > 0 {
		u.Texture = cs.Texture
	}
	if len(cs.TextureHash) > 0 {
		u.TextureHash = cs.TextureHash
	}
	if cs.CertificateHash != "" {
		u.CertificateHash = cs.CertificateHash
	}
	if cs.PluginIdentity != "" {
		u.PluginIdentity = cs.PluginIdentity
	}
	if len(cs.PluginContext) > 0 {
		u.PluginContext = cs.PluginContext
	}

	s.Users[cs.Session] = u
	if s.Self != nil && s.Self.Session == cs.Session {
		s.Self = &u
	}
}

// UpsertChannelFromProto parses a ChannelState protobuf message and upserts it.
func (s *Store) UpsertChannelFromProto(cs *protocol.ChannelState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := s.Channels[cs.ChannelID]
	if cs.HasChannelID {
		ch.ID = cs.ChannelID
	}
	if cs.HasParent {
		ch.ParentID = cs.Parent
	}
	if cs.Name != "" {
		ch.Name = cs.Name
	}
	ch.Position = cs.Position
	ch.Links = cs.Links
	if cs.Description != "" {
		ch.Description = cs.Description
	}
	ch.Temporary = cs.Temporary
	ch.MaxUsers = cs.MaxUsers

	s.Channels[cs.ChannelID] = ch
}

// RemoveUser removes a user from the store.
func (s *Store) RemoveUser(session uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Users, session)
	if s.Self != nil && s.Self.Session == session {
		s.Self = nil
	}
}

// RemoveChannel removes a channel from the store.
func (s *Store) RemoveChannel(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Channels, id)
}

// SetSelf updates the current client user.
func (s *Store) SetSelf(u User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := u
	s.Self = &copy
	s.Users[u.Session] = u
}
