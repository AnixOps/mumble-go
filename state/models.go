package state

// User is the normalized user state exposed by the client.
type User struct {
	Session           uint32
	UserID            uint32
	Name              string
	ChannelID         uint32
	HasChannelID      bool
	Muted             bool
	Deafened          bool
	SelfMute          bool
	SelfDeaf          bool
	Suppress          bool
	Recording         bool

	// Extended
	Texture         []byte
	TextureHash     []byte
	CertificateHash string
	PluginIdentity  string
	PluginContext   []byte
}

// Channel is the normalized channel state exposed by the client.
type Channel struct {
	ID       uint32
	ParentID uint32
	Name     string
	Position int32
	Links    []uint32
	Description string
	Temporary bool
	MaxUsers  uint32
}
