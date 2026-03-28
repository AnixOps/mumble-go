package protocol

import (
	"bytes"
	"testing"
)

func TestVersionMarshalUnmarshal(t *testing.T) {
	v := &Version{
		Version:   0x010203,
		Release:   "mumble-go-test",
		OS:        "Linux",
		OSVersion: "x86_64",
	}
	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	v2 := &Version{}
	if err := v2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if v2.Version != v.Version || v2.Release != v.Release || v2.OS != v.OS || v2.OSVersion != v.OSVersion {
		t.Errorf("round-trip mismatch: got %+v, want %+v", v2, v)
	}
}

func TestAuthenticateMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		a       *Authenticate
		wantErr bool
	}{
		{
			name: "full",
			a: &Authenticate{
				Username: "testuser",
				Password: "secret",
				Tokens:   []string{"token1", "token2"},
				Opus:     true,
				Ping:     "pingdata",
			},
		},
		{
			name: "minimal",
			a: &Authenticate{
				Username: "user",
			},
		},
		{
			name: "empty",
			a:    &Authenticate{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.a.Marshal()
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			a2 := &Authenticate{}
			if err := a2.Unmarshal(data); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !tt.wantErr && (a2.Username != tt.a.Username || a2.Password != tt.a.Password || a2.Opus != tt.a.Opus || len(a2.Tokens) != len(tt.a.Tokens)) {
				t.Errorf("round-trip mismatch: got %+v, want %+v", a2, tt.a)
			}
		})
	}
}

func TestServerSyncMarshalUnmarshal(t *testing.T) {
	ss := &ServerSync{
		Session:        42,
		MaxBandwidth:   72000,
		WelcomeText:    "Welcome to Mumble",
		ServerIDHash:   "sha256:abc123",
		WelcomeText2:   "HTML welcome",
	}
	data, err := ss.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	ss2 := &ServerSync{}
	if err := ss2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if ss2.Session != ss.Session || ss2.MaxBandwidth != ss.MaxBandwidth || ss2.WelcomeText != ss.WelcomeText {
		t.Errorf("round-trip mismatch: got %+v, want %+v", ss2, ss)
	}
}

func TestChannelStateMarshalUnmarshal(t *testing.T) {
	cs := &ChannelState{
		ChannelID:   5,
		HasChannelID: true,
		Parent:      2,
		HasParent:   true,
		Name:        "Test Channel",
		Links:       []uint32{1, 3, 4},
		Description: "A test channel",
		Position:    10,
		MaxUsers:    100,
		Temporary:   false,
	}
	data, err := cs.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	cs2 := &ChannelState{}
	if err := cs2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if cs2.ChannelID != cs.ChannelID || cs2.Parent != cs.Parent || cs2.Name != cs.Name || len(cs2.Links) != len(cs.Links) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", cs2, cs)
	}
}

func TestUserStateMarshalUnmarshal(t *testing.T) {
	us := &UserState{
		Session:     42,
		Actor:       1,
		Name:        "TestUser",
		UserID:      1,
		ChannelID:   5,
		HasChannelID: true,
		Mute:        true,
		Deaf:        false,
		Suppress:    false,
		SelfMute:    true,
		SelfDeaf:    false,
		Recording:   false,
		Comment:     "hello",
		CertificateHash: "deadbeef",
	}
	data, err := us.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	us2 := &UserState{}
	if err := us2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if us2.Session != us.Session || us2.Name != us.Name || us2.Mute != us.Mute || us2.SelfMute != us.SelfMute {
		t.Errorf("round-trip mismatch: got %+v, want %+v", us2, us)
	}
}

func TestRejectMarshalUnmarshal(t *testing.T) {
	r := &Reject{
		Reason: "Invalid username or password",
		Type:   "PermissionDenied",
	}
	data, err := r.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	r2 := &Reject{}
	if err := r2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if r2.Reason != r.Reason || r2.Type != r.Type {
		t.Errorf("round-trip mismatch: got %+v, want %+v", r2, r)
	}
}

func TestPermissionDeniedMarshalUnmarshal(t *testing.T) {
	pd := &PermissionDenied{
		Session:    6,
		Type:       7,
		ChannelID:  1,
		Permission: 4,
		Reason:     "missing cert",
		Name:       "test",
	}
	data, err := pd.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	pd2 := &PermissionDenied{}
	if err := pd2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if pd2.Session != pd.Session || pd2.Type != pd.Type || pd2.ChannelID != pd.ChannelID || pd2.Reason != pd.Reason || pd2.Name != pd.Name {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", pd2, pd)
	}
	if got := pd2.TypeString(); got != "MissingCertificate" {
		t.Fatalf("TypeString mismatch: got %q", got)
	}
}

func TestCodecVersionMarshalUnmarshal(t *testing.T) {
	cv := &CodecVersion{
		Alpha:        -1,
		Beta:         -1,
		Opus:         true,
		OpusSupport:  true,
		OpusMessages: true,
	}
	data, err := cv.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	cv2 := &CodecVersion{}
	if err := cv2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if cv2.Opus != cv.Opus || cv2.OpusSupport != cv.OpusSupport {
		t.Errorf("round-trip mismatch: got %+v, want %+v", cv2, cv)
	}
}

func TestPingMarshalUnmarshal(t *testing.T) {
	p := &Ping{
		Timestamp:  1234567890,
		NumPackets: 100,
		NumBytes:   50000,
		UdpPing:    12.5,
		TcpPing:    15.3,
	}
	data, err := p.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	p2 := &Ping{}
	if err := p2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if p2.Timestamp != p.Timestamp || p2.NumPackets != p.NumPackets || p2.UdpPing != p.UdpPing {
		t.Errorf("round-trip mismatch: got %+v, want %+v", p2, p)
	}
}

func TestUserRemoveMarshalUnmarshal(t *testing.T) {
	ur := &UserRemove{
		Session: 42,
		Actor:   1,
		Reason:  "User left",
	}
	data, err := ur.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	ur2 := &UserRemove{}
	if err := ur2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if ur2.Session != ur.Session || ur2.Reason != ur.Reason {
		t.Errorf("round-trip mismatch: got %+v, want %+v", ur2, ur)
	}
}

func TestServerConfigMarshalUnmarshal(t *testing.T) {
	sc := &ServerConfig{
		MaxBandwidth:       72000,
		WelcomeText:        "Welcome",
		WelcomeText2:       "<b>Welcome</b>",
		AllowHTML:          true,
		MessageLength:      5000,
		ImageMessageLength: 100000,
	}
	data, err := sc.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	sc2 := &ServerConfig{}
	if err := sc2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if sc2.MaxBandwidth != sc.MaxBandwidth || sc2.AllowHTML != sc.AllowHTML {
		t.Errorf("round-trip mismatch: got %+v, want %+v", sc2, sc)
	}
}

func TestTextMessageMarshalUnmarshal(t *testing.T) {
	tm := &TextMessage{
		Actor:     42,
		Session:   []uint32{1, 2, 3},
		ChannelID: []uint32{5, 6},
		TreeID:    []uint32{},
		Message:   "Hello, world!",
	}
	data, err := tm.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	tm2 := &TextMessage{}
	if err := tm2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if tm2.Actor != tm.Actor || tm2.Message != tm.Message || len(tm2.Session) != len(tm.Session) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", tm2, tm)
	}
}

func TestChannelRemoveMarshalUnmarshal(t *testing.T) {
	cr := &ChannelRemove{ChannelID: 42}
	data, err := cr.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	cr2 := &ChannelRemove{}
	if err := cr2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if cr2.ChannelID != cr.ChannelID {
		t.Errorf("round-trip mismatch: got %+v, want %+v", cr2, cr)
	}
}

func TestCryptSetupMarshalUnmarshal(t *testing.T) {
	cs := &CryptSetup{
		Key:         []byte("0123456789abcdef0123456789abcdef"),
		ClientNonce: []byte("clientnonce012345678901234567"),
		ServerNonce: []byte("servernonce01234567890123456"),
	}
	data, err := cs.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	cs2 := &CryptSetup{}
	if err := cs2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !bytes.Equal(cs2.Key, cs.Key) || !bytes.Equal(cs2.ClientNonce, cs.ClientNonce) || !bytes.Equal(cs2.ServerNonce, cs.ServerNonce) {
		t.Errorf("round-trip mismatch: got %x/%x/%x, want %x/%x/%x", cs2.Key, cs2.ClientNonce, cs2.ServerNonce, cs.Key, cs.ClientNonce, cs.ServerNonce)
	}
}

func TestPingTruncated(t *testing.T) {
	// Test that truncated ping doesn't panic
	p := &Ping{}
	if err := p.Unmarshal([]byte{0x08}); err != nil {
		t.Errorf("Unmarshal of truncated data should not error, got: %v", err)
	}
}

func TestVersionTruncated(t *testing.T) {
	// Test that truncated version doesn't panic
	v := &Version{}
	if err := v.Unmarshal([]byte{0x08}); err != nil {
		t.Errorf("Unmarshal of truncated data should not error, got: %v", err)
	}
}

func TestAppendVarint(t *testing.T) {
	tests := []struct {
		val    uint64
		expect []byte
	}{
		{0, []byte{0}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{300, []byte{0xac, 0x02}},
		{0xFFFFFFFFFFFFFFFF, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
	}
	for _, tt := range tests {
		var b []byte
		b = appendVarint(b, tt.val)
		if !bytes.Equal(b, tt.expect) {
			t.Errorf("appendVarint(%d) = %x, want %x", tt.val, b, tt.expect)
		}
	}
}
