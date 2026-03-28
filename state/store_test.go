package state

import (
	"testing"

	"mumble-go/protocol"
)

func TestStoreNew(t *testing.T) {
	s := NewStore()
	if s == nil {
		t.Fatal("NewStore returned nil")
	}
	if s.SelfSession() != 0 {
		t.Errorf("new store should have SelfSession=0, got %d", s.SelfSession())
	}
	if s.IsReady() {
		t.Errorf("new store should not be ready")
	}
}

func TestStoreSnapshotUsers(t *testing.T) {
	s := NewStore()
	users := s.SnapshotUsers()
	if len(users) != 0 {
		t.Errorf("new store should have 0 users, got %d", len(users))
	}
}

func TestStoreSnapshotChannels(t *testing.T) {
	s := NewStore()
	channels := s.SnapshotChannels()
	if len(channels) != 0 {
		t.Errorf("new store should have 0 channels, got %d", len(channels))
	}
}

func TestStoreUpsertUser(t *testing.T) {
	s := NewStore()
	us := &protocol.UserState{
		Session:   42,
		Name:      "testuser",
		ChannelID: 1,
		Mute:      true,
	}
	s.UpsertUserFromProto(us)
	users := s.SnapshotUsers()
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}
	if users[42].Name != "testuser" {
		t.Errorf("user name mismatch: got %s, want testuser", users[42].Name)
	}
}

func TestStoreUpsertChannel(t *testing.T) {
	s := NewStore()
	ch := &protocol.ChannelState{
		ChannelID: 5,
		Parent:    1,
		Name:      "TestChannel",
		Position:  10,
	}
	s.UpsertChannelFromProto(ch)
	channels := s.SnapshotChannels()
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}
	if channels[5].Name != "TestChannel" {
		t.Errorf("channel name mismatch: got %s, want TestChannel", channels[5].Name)
	}
}

func TestStoreRemoveUser(t *testing.T) {
	s := NewStore()
	us := &protocol.UserState{Session: 42, Name: "test"}
	s.UpsertUserFromProto(us)
	if len(s.SnapshotUsers()) != 1 {
		t.Fatal("expected 1 user before removal")
	}
	s.RemoveUser(42)
	if len(s.SnapshotUsers()) != 0 {
		t.Errorf("expected 0 users after removal, got %d", len(s.SnapshotUsers()))
	}
}

func TestStoreRemoveChannel(t *testing.T) {
	s := NewStore()
	ch := &protocol.ChannelState{ChannelID: 5, Name: "test"}
	s.UpsertChannelFromProto(ch)
	if len(s.SnapshotChannels()) != 1 {
		t.Fatal("expected 1 channel before removal")
	}
	s.RemoveChannel(5)
	if len(s.SnapshotChannels()) != 0 {
		t.Errorf("expected 0 channels after removal, got %d", len(s.SnapshotChannels()))
	}
}

func TestStoreSetSelf(t *testing.T) {
	s := NewStore()
	user := User{Session: 99, Name: "self"}
	s.SetSelf(user)
	if s.SelfSession() != 99 {
		t.Errorf("SelfSession = %d, want 99", s.SelfSession())
	}
}

func TestStoreMarkReady(t *testing.T) {
	s := NewStore()
	if s.IsReady() {
		t.Error("new store should not be ready")
	}
	s.MarkReady()
	if !s.IsReady() {
		t.Error("store should be ready after MarkReady")
	}
}
