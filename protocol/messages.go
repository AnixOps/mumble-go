package protocol

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
)

// Mumble protocol message structures.
// Binary encoding follows standard protobuf rules.

const (
	wireVarint     = 0
	wireFixed64    = 1
	wireLengthDelim = 2
	wireFixed32    = 5
)

// EnableDebug causes protocol message decode errors to be logged instead of
// causing panics. Useful when debugging unknown server messages.
var EnableDebug = false

func debug(format string, args ...interface{}) {
	if EnableDebug {
		log.Printf(format, args...)
	}
}

// DebugFrame hex-dumps a frame if debug logging is enabled.
func DebugFrame(t MessageType, payload []byte) {
	if !EnableDebug {
		return
	}
	if len(payload) > 64 {
		payload = payload[:64]
	}
	hex := fmt.Sprintf("% x", payload)
	log.Printf("[frame] type=%d len=%d payload_hex=%s", t, len(payload), hex)
}

// --- Version ---

// Version is sent by both sides to announce their version.
type Version struct {
	Version   uint32
	Release   string
	OS        string
	OSVersion string
}

func (v *Version) Marshal() ([]byte, error) {
	var b []byte
	if v.Version != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(v.Version))
	}
	if v.Release != "" {
		b = append(b, 0x12)
		b = appendVarint(b, uint64(len(v.Release)))
		b = append(b, v.Release...)
	}
	if v.OS != "" {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(v.OS)))
		b = append(b, v.OS...)
	}
	if v.OSVersion != "" {
		b = append(b, 0x22)
		b = appendVarint(b, uint64(len(v.OSVersion)))
		b = append(b, v.OSVersion...)
	}
	return b, nil
}

func (v *Version) Unmarshal(b []byte) error {
	v.Version = 0
	v.Release = ""
	v.OS = ""
	v.OSVersion = ""
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			v.Version = uint32(val)
		case 2:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			v.Release = string(b[i : i+int(n)])
			i += int(n)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			v.OS = string(b[i : i+int(n)])
			i += int(n)
		case 4:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			v.OSVersion = string(b[i : i+int(n)])
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- Authenticate ---

// Authenticate is sent by the client after Version.
type Authenticate struct {
	Username     string
	Password     string
	Tokens       []string
	Opus         bool
	CeltVersions []int32
	Ping         string
}

func (a *Authenticate) Marshal() ([]byte, error) {
	var b []byte
	if a.Username != "" {
		b = append(b, 0x0a)
		b = appendVarint(b, uint64(len(a.Username)))
		b = append(b, a.Username...)
	}
	if a.Password != "" {
		b = append(b, 0x12)
		b = appendVarint(b, uint64(len(a.Password)))
		b = append(b, a.Password...)
	}
	for _, tok := range a.Tokens {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(tok)))
		b = append(b, tok...)
	}
	if a.Opus {
		b = append(b, 0x20)
		b = appendVarint(b, 1)
	}
	if a.Ping != "" {
		b = append(b, 0x32)
		b = appendVarint(b, uint64(len(a.Ping)))
		b = append(b, a.Ping...)
	}
	return b, nil
}

func (a *Authenticate) Unmarshal(b []byte) error {
	a.Username = ""
	a.Password = ""
	a.Tokens = nil
	a.Opus = false
	a.Ping = ""
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			a.Username = string(b[i : i+int(n)])
			i += int(n)
		case 2:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			a.Password = string(b[i : i+int(n)])
			i += int(n)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			a.Tokens = append(a.Tokens, string(b[i:i+int(n)]))
			i += int(n)
		case 4:
			var val uint64
			val, i = readVarintSafe(b, i)
			a.Opus = val != 0
		case 6:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			a.Ping = string(b[i : i+int(n)])
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- ServerSync ---

// ServerSync is sent by the server after Authenticate.
type ServerSync struct {
	Session        uint64
	MaxBandwidth   uint32
	WelcomeText    string
	ServerIDHash   string
	WelcomeText2   string
}

func (s *ServerSync) Marshal() ([]byte, error) {
	var b []byte
	if s.Session != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, s.Session)
	}
	if s.MaxBandwidth != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(s.MaxBandwidth))
	}
	if s.WelcomeText != "" {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(s.WelcomeText)))
		b = append(b, s.WelcomeText...)
	}
	if s.ServerIDHash != "" {
		b = append(b, 0x22)
		b = appendVarint(b, uint64(len(s.ServerIDHash)))
		b = append(b, s.ServerIDHash...)
	}
	if s.WelcomeText2 != "" {
		b = append(b, 0x2a)
		b = appendVarint(b, uint64(len(s.WelcomeText2)))
		b = append(b, s.WelcomeText2...)
	}
	return b, nil
}

func (s *ServerSync) Unmarshal(b []byte) error {
	s.Session = 0
	s.MaxBandwidth = 0
	s.WelcomeText = ""
	s.ServerIDHash = ""
	s.WelcomeText2 = ""
	i := 0
	for i < len(b) {
		if i >= len(b) {
			break
		}
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			s.Session = val
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			s.MaxBandwidth = uint32(val)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n > 0 && i+int(n) <= len(b) {
				s.WelcomeText = string(b[i : i+int(n)])
				i += int(n)
			}
		case 4:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n > 0 && i+int(n) <= len(b) {
				s.ServerIDHash = string(b[i : i+int(n)])
				i += int(n)
			}
		case 5:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n > 0 && i+int(n) <= len(b) {
				s.WelcomeText2 = string(b[i : i+int(n)])
				i += int(n)
			}
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- ChannelState ---

// ChannelState describes a channel.
type ChannelState struct {
	ChannelID          uint32
	HasChannelID       bool
	Parent             uint32
	HasParent          bool
	Name               string
	Links              []uint32
	Description        string
	LinksAdd           []uint32
	LinksRemove        []uint32
	Temporary          bool
	Position           int32
	DescriptionHash    []byte
	MaxUsers           uint32
	IsEnterRestricted  bool
	CanEnter           bool
}

func (c *ChannelState) Marshal() ([]byte, error) {
	var b []byte
	if c.HasChannelID || c.ChannelID != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(c.ChannelID))
	}
	if c.HasParent || c.Parent != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(c.Parent))
	}
	if c.Name != "" {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(c.Name)))
		b = append(b, c.Name...)
	}
	for _, l := range c.Links {
		b = append(b, 0x20)
		b = appendVarint(b, uint64(l))
	}
	if c.Description != "" {
		b = append(b, 0x2a)
		b = appendVarint(b, uint64(len(c.Description)))
		b = append(b, c.Description...)
	}
	for _, l := range c.LinksAdd {
		b = append(b, 0x30)
		b = appendVarint(b, uint64(l))
	}
	for _, l := range c.LinksRemove {
		b = append(b, 0x38)
		b = appendVarint(b, uint64(l))
	}
	if c.Temporary {
		b = append(b, 0x40)
		b = appendVarint(b, 1)
	}
	if c.Position != 0 {
		b = append(b, 0x48)
		b = appendVarint(b, uint64(c.Position))
	}
	if len(c.DescriptionHash) > 0 {
		b = append(b, 0x52)
		b = appendVarint(b, uint64(len(c.DescriptionHash)))
		b = append(b, c.DescriptionHash...)
	}
	if c.MaxUsers != 0 {
		b = append(b, 0x58)
		b = appendVarint(b, uint64(c.MaxUsers))
	}
	if c.IsEnterRestricted {
		b = append(b, 0x60)
		b = appendVarint(b, 1)
	}
	if c.CanEnter {
		b = append(b, 0x68)
		b = appendVarint(b, 1)
	}
	return b, nil
}

func (c *ChannelState) Unmarshal(b []byte) error {
	*c = ChannelState{}
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.ChannelID = uint32(val)
			c.HasChannelID = true
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Parent = uint32(val)
			c.HasParent = true
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			c.Name = string(b[i : i+int(n)])
			i += int(n)
		case 4:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Links = append(c.Links, uint32(val))
		case 5:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			c.Description = string(b[i : i+int(n)])
			i += int(n)
		case 6:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.LinksAdd = append(c.LinksAdd, uint32(val))
		case 7:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.LinksRemove = append(c.LinksRemove, uint32(val))
		case 8:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Temporary = val != 0
		case 9:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Position = int32(val)
		case 10:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			c.DescriptionHash = b[i : i+int(n)]
			i += int(n)
		case 11:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.MaxUsers = uint32(val)
		case 12:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.IsEnterRestricted = val != 0
		case 13:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.CanEnter = val != 0
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- UserState ---

// UserState describes a connected user.
type UserState struct {
	Session               uint32
	Actor                 uint32
	Name                  string
	UserID                uint32
	IncludeUserID         bool
	ChannelID             uint32
	HasChannelID          bool
	Mute                  bool
	Deaf                  bool
	Suppress              bool
	SelfMute              bool
	SelfDeaf              bool
	Texture               []byte
	PluginContext         []byte
	PluginIdentity        string
	Comment               string
	CertificateHash       string
	CommentHash           []byte
	TextureHash           []byte
	PrioritySpeaker       bool
	Recording             bool
	TemporaryAccessTokens []string
	ListeningChannelAdd   []uint32
	ListeningChannelRemove []uint32
}

func (u *UserState) Marshal() ([]byte, error) {
	var b []byte
	if u.Session != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(u.Session))
	}
	if u.Actor != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(u.Actor))
	}
	if u.Name != "" {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(u.Name)))
		b = append(b, u.Name...)
	}
	if u.IncludeUserID {
		b = append(b, 0x20)
		b = appendVarint(b, uint64(u.UserID))
	}
	if u.HasChannelID || u.ChannelID != 0 {
		b = append(b, 0x28)
		b = appendVarint(b, uint64(u.ChannelID))
	}
	if u.Mute {
		b = append(b, 0x30)
		b = appendVarint(b, 1)
	}
	if u.Deaf {
		b = append(b, 0x38)
		b = appendVarint(b, 1)
	}
	if u.Suppress {
		b = append(b, 0x40)
		b = appendVarint(b, 1)
	}
	if u.SelfMute {
		b = append(b, 0x48)
		b = appendVarint(b, 1)
	}
	if u.SelfDeaf {
		b = append(b, 0x50)
		b = appendVarint(b, 1)
	}
	if len(u.Texture) > 0 {
		b = append(b, 0x5a)
		b = appendVarint(b, uint64(len(u.Texture)))
		b = append(b, u.Texture...)
	}
	if len(u.PluginContext) > 0 {
		b = append(b, 0x62)
		b = appendVarint(b, uint64(len(u.PluginContext)))
		b = append(b, u.PluginContext...)
	}
	if u.PluginIdentity != "" {
		b = append(b, 0x6a)
		b = appendVarint(b, uint64(len(u.PluginIdentity)))
		b = append(b, u.PluginIdentity...)
	}
	if u.Comment != "" {
		b = append(b, 0x72)
		b = appendVarint(b, uint64(len(u.Comment)))
		b = append(b, u.Comment...)
	}
	if u.CertificateHash != "" {
		b = append(b, 0x7a)
		b = appendVarint(b, uint64(len(u.CertificateHash)))
		b = append(b, u.CertificateHash...)
	}
	if len(u.CommentHash) > 0 {
		b = append(b, 0x82, 0x01)
		b = appendVarint(b, uint64(len(u.CommentHash)))
		b = append(b, u.CommentHash...)
	}
	if len(u.TextureHash) > 0 {
		b = append(b, 0x8a, 0x01)
		b = appendVarint(b, uint64(len(u.TextureHash)))
		b = append(b, u.TextureHash...)
	}
	if u.PrioritySpeaker {
		b = append(b, 0x90, 0x01)
		b = appendVarint(b, 1)
	}
	if u.Recording {
		b = append(b, 0x98, 0x01)
		b = appendVarint(b, 1)
	}
	for _, tok := range u.TemporaryAccessTokens {
		b = append(b, 0xa2, 0x01)
		b = appendVarint(b, uint64(len(tok)))
		b = append(b, tok...)
	}
	for _, ch := range u.ListeningChannelAdd {
		b = append(b, 0xa8, 0x01)
		b = appendVarint(b, uint64(ch))
	}
	for _, ch := range u.ListeningChannelRemove {
		b = append(b, 0xb0, 0x01)
		b = appendVarint(b, uint64(ch))
	}
	return b, nil
}

func (u *UserState) Unmarshal(b []byte) error {
	*u = UserState{}
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Session = uint32(val)
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Actor = uint32(val)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.Name = string(b[i : i+int(n)])
			i += int(n)
		case 4:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.UserID = uint32(val)
		case 5:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.ChannelID = uint32(val)
			u.HasChannelID = true
		case 6:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Mute = val != 0
		case 7:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Deaf = val != 0
		case 8:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Suppress = val != 0
		case 9:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.SelfMute = val != 0
		case 10:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.SelfDeaf = val != 0
		case 11:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.Texture = b[i : i+int(n)]
			i += int(n)
		case 12:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.PluginContext = b[i : i+int(n)]
			i += int(n)
		case 13:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.PluginIdentity = string(b[i : i+int(n)])
			i += int(n)
		case 14:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.Comment = string(b[i : i+int(n)])
			i += int(n)
		case 15:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.CertificateHash = string(b[i : i+int(n)])
			i += int(n)
		case 16:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.CommentHash = b[i : i+int(n)]
			i += int(n)
		case 17:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.TextureHash = b[i : i+int(n)]
			i += int(n)
		case 18:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.PrioritySpeaker = val != 0
		case 19:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Recording = val != 0
		case 20:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.TemporaryAccessTokens = append(u.TemporaryAccessTokens, string(b[i:i+int(n)]))
			i += int(n)
		case 21:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.ListeningChannelAdd = append(u.ListeningChannelAdd, uint32(val))
		case 22:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.ListeningChannelRemove = append(u.ListeningChannelRemove, uint32(val))
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- Reject ---

// Reject is sent by the server when rejecting a connection.
type Reject struct {
	Reason string
	Type   string
}

type PermissionDenied struct {
	Permission uint32
	ChannelID  uint32
	Session    uint32
	Reason     string
	Type       uint32
	Name       string
}

func (p *PermissionDenied) Marshal() ([]byte, error) {
	var b []byte
	if p.Permission != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(p.Permission))
	}
	if p.ChannelID != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(p.ChannelID))
	}
	if p.Session != 0 {
		b = append(b, 0x18)
		b = appendVarint(b, uint64(p.Session))
	}
	if p.Reason != "" {
		b = append(b, 0x22)
		b = appendVarint(b, uint64(len(p.Reason)))
		b = append(b, p.Reason...)
	}
	if p.Type != 0 {
		b = append(b, 0x28)
		b = appendVarint(b, uint64(p.Type))
	}
	if p.Name != "" {
		b = append(b, 0x32)
		b = appendVarint(b, uint64(len(p.Name)))
		b = append(b, p.Name...)
	}
	return b, nil
}

func (p *PermissionDenied) Unmarshal(b []byte) error {
	*p = PermissionDenied{}
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.Permission = uint32(val)
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.ChannelID = uint32(val)
		case 3:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.Session = uint32(val)
		case 4:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			p.Reason = string(b[i : i+int(n)])
			i += int(n)
		case 5:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.Type = uint32(val)
		case 6:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			p.Name = string(b[i : i+int(n)])
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

func (p *PermissionDenied) TypeString() string {
	switch p.Type {
	case 1:
		return "Text"
	case 2:
		return "Permission"
	case 3:
		return "SuperUser"
	case 4:
		return "ChannelName"
	case 5:
		return "TextTooLong"
	case 6:
		return "TemporaryChannel"
	case 7:
		return "MissingCertificate"
	case 8:
		return "UserName"
	case 9:
		return "ChannelFull"
	case 10:
		return "NestingLimit"
	case 11:
		return "ChannelCountLimit"
	case 12:
		return "ChannelListenerLimit"
	case 13:
		return "UserListenerLimit"
	default:
		return "Unknown"
	}
}

func (p *PermissionDenied) String() string {
	perm := permissionName(p.Permission)
	if p.Reason != "" {
		return fmt.Sprintf("type=%s(%d) session=%d channel=%d permission=%s(%d) reason=%q", p.TypeString(), p.Type, p.Session, p.ChannelID, perm, p.Permission, p.Reason)
	}
	return fmt.Sprintf("type=%s(%d) session=%d channel=%d permission=%s(%d) name=%q", p.TypeString(), p.Type, p.Session, p.ChannelID, perm, p.Permission, p.Name)
}

func permissionName(v uint32) string {
	switch v {
	case 0:
		return "None"
	case 1:
		return "Write"
	case 2:
		return "Traverse"
	case 4:
		return "Enter"
	case 8:
		return "Speak"
	case 16:
		return "MuteDeafen"
	case 32:
		return "Move"
	case 64:
		return "MakeChannel"
	case 128:
		return "LinkChannel"
	case 256:
		return "Whisper"
	case 512:
		return "TextMessage"
	case 1024:
		return "MakeTempChannel"
	case 65536:
		return "Listen"
	case 131072:
		return "Kick"
	case 262144:
		return "Ban"
	case 524288:
		return "Register"
	case 1048576:
		return "SelfRegister"
	default:
		return "Unknown"
	}
}

func (r *Reject) Marshal() ([]byte, error) {
	var b []byte
	if r.Reason != "" {
		b = append(b, 0x0a)
		b = appendVarint(b, uint64(len(r.Reason)))
		b = append(b, r.Reason...)
	}
	if r.Type != "" {
		b = append(b, 0x12)
		b = appendVarint(b, uint64(len(r.Type)))
		b = append(b, r.Type...)
	}
	return b, nil
}

func (r *Reject) Unmarshal(b []byte) error {
	r.Reason = ""
	r.Type = ""
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			r.Reason = string(b[i : i+int(n)])
			i += int(n)
		case 2:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			r.Type = string(b[i : i+int(n)])
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- CodecVersion ---

// CodecVersion announces server-supported audio codecs.
type CodecVersion struct {
	Alpha        int32
	Beta         int32
	Opus         bool
	OpusSupport  bool
	OpusMessages bool
}

func (c *CodecVersion) Marshal() ([]byte, error) {
	var b []byte
	if c.Alpha != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(c.Alpha))
	}
	if c.Beta != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(c.Beta))
	}
	if c.Opus {
		b = append(b, 0x18)
		b = appendVarint(b, 1)
	}
	if c.OpusSupport {
		b = append(b, 0x20)
		b = appendVarint(b, 1)
	}
	if c.OpusMessages {
		b = append(b, 0x28)
		b = appendVarint(b, 1)
	}
	return b, nil
}

func (c *CodecVersion) Unmarshal(b []byte) error {
	c.Alpha = 0
	c.Beta = 0
	c.Opus = false
	c.OpusSupport = false
	c.OpusMessages = false
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Alpha = int32(val)
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Beta = int32(val)
		case 3:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.Opus = val != 0
		case 4:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.OpusSupport = val != 0
		case 5:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.OpusMessages = val != 0
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- Ping ---

// Ping is a keepalive message.
type Ping struct {
	Timestamp  uint64
	NumPackets uint32
	NumBytes   uint64
	UdpPing    float64
	TcpPing    float64
}

func (p *Ping) Marshal() ([]byte, error) {
	var b []byte
	if p.Timestamp != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, p.Timestamp)
	}
	if p.NumPackets != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(p.NumPackets))
	}
	if p.NumBytes != 0 {
		b = append(b, 0x18)
		b = appendVarint(b, p.NumBytes)
	}
	if p.UdpPing != 0 {
		b = append(b, 0x21)
		b = appendFixed64(b, math.Float64bits(p.UdpPing))
	}
	if p.TcpPing != 0 {
		b = append(b, 0x29)
		b = appendFixed64(b, math.Float64bits(p.TcpPing))
	}
	return b, nil
}

func (p *Ping) Unmarshal(b []byte) error {
	p.Timestamp = 0
	p.NumPackets = 0
	p.NumBytes = 0
	p.UdpPing = 0
	p.TcpPing = 0
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.Timestamp = val
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.NumPackets = uint32(val)
		case 3:
			var val uint64
			val, i = readVarintSafe(b, i)
			p.NumBytes = val
		case 4:
			if i+8 > len(b) {
				break
			}
			p.UdpPing = math.Float64frombits(binary.LittleEndian.Uint64(b[i : i+8]))
			i += 8
		case 5:
			if i+8 > len(b) {
				break
			}
			p.TcpPing = math.Float64frombits(binary.LittleEndian.Uint64(b[i : i+8]))
			i += 8
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- UserRemove ---

// UserRemove is sent by the server when a user leaves or is kicked/banned.
type UserRemove struct {
	Session uint32
	Actor   uint32
	Reason  string
}

func (u *UserRemove) Marshal() ([]byte, error) {
	var b []byte
	if u.Session != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(u.Session))
	}
	if u.Actor != 0 {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(u.Actor))
	}
	if u.Reason != "" {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(u.Reason)))
		b = append(b, u.Reason...)
	}
	return b, nil
}

func (u *UserRemove) Unmarshal(b []byte) error {
	u.Session = 0
	u.Actor = 0
	u.Reason = ""
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Session = uint32(val)
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			u.Actor = uint32(val)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			u.Reason = string(b[i : i+int(n)])
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- ServerConfig ---

// ServerConfig carries server settings sent after ServerSync.
type ServerConfig struct {
	MaxBandwidth       uint32
	WelcomeText        string
	WelcomeText2       string
	AllowHTML          bool
	MessageLength      uint32
	ImageMessageLength uint32
}

func (s *ServerConfig) Marshal() ([]byte, error) {
	var b []byte
	if s.MaxBandwidth != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(s.MaxBandwidth))
	}
	if s.WelcomeText != "" {
		b = append(b, 0x12)
		b = appendVarint(b, uint64(len(s.WelcomeText)))
		b = append(b, s.WelcomeText...)
	}
	if s.WelcomeText2 != "" {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(s.WelcomeText2)))
		b = append(b, s.WelcomeText2...)
	}
	if s.AllowHTML {
		b = append(b, 0x20)
		b = appendVarint(b, 1)
	}
	if s.MessageLength != 0 {
		b = append(b, 0x28)
		b = appendVarint(b, uint64(s.MessageLength))
	}
	if s.ImageMessageLength != 0 {
		b = append(b, 0x30)
		b = appendVarint(b, uint64(s.ImageMessageLength))
	}
	return b, nil
}

func (s *ServerConfig) Unmarshal(b []byte) error {
	s.MaxBandwidth = 0
	s.WelcomeText = ""
	s.WelcomeText2 = ""
	s.AllowHTML = false
	s.MessageLength = 0
	s.ImageMessageLength = 0
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			s.MaxBandwidth = uint32(val)
		case 2:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			s.WelcomeText = string(b[i : i+int(n)])
			i += int(n)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			s.WelcomeText2 = string(b[i : i+int(n)])
			i += int(n)
		case 4:
			var val uint64
			val, i = readVarintSafe(b, i)
			s.AllowHTML = val != 0
		case 5:
			var val uint64
			val, i = readVarintSafe(b, i)
			s.MessageLength = uint32(val)
		case 6:
			var val uint64
			val, i = readVarintSafe(b, i)
			s.ImageMessageLength = uint32(val)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- TextMessage ---

// TextMessage is a chat message.
type TextMessage struct {
	Actor     uint32
	Session   []uint32
	ChannelID []uint32
	TreeID    []uint32
	Message   string
}

func (t *TextMessage) Marshal() ([]byte, error) {
	var b []byte
	if t.Actor != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(t.Actor))
	}
	for _, s := range t.Session {
		b = append(b, 0x10)
		b = appendVarint(b, uint64(s))
	}
	for _, c := range t.ChannelID {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(c))
	}
	for _, tr := range t.TreeID {
		b = append(b, 0x22)
		b = appendVarint(b, uint64(tr))
	}
	if t.Message != "" {
		b = append(b, 0x2a)
		b = appendVarint(b, uint64(len(t.Message)))
		b = append(b, t.Message...)
	}
	return b, nil
}

func (t *TextMessage) Unmarshal(b []byte) error {
	t.Actor = 0
	t.Session = nil
	t.ChannelID = nil
	t.TreeID = nil
	t.Message = ""
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			t.Actor = uint32(val)
		case 2:
			var val uint64
			val, i = readVarintSafe(b, i)
			t.Session = append(t.Session, uint32(val))
		case 3:
			var val uint64
			val, i = readVarintSafe(b, i)
			t.ChannelID = append(t.ChannelID, uint32(val))
		case 4:
			var val uint64
			val, i = readVarintSafe(b, i)
			t.TreeID = append(t.TreeID, uint32(val))
		case 5:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			t.Message = string(b[i : i+int(n)])
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- ChannelRemove ---

// ChannelRemove is sent by the server when a channel is removed.
type ChannelRemove struct {
	ChannelID uint32
}

func (c *ChannelRemove) Marshal() ([]byte, error) {
	var b []byte
	if c.ChannelID != 0 {
		b = append(b, 0x08)
		b = appendVarint(b, uint64(c.ChannelID))
	}
	return b, nil
}

func (c *ChannelRemove) Unmarshal(b []byte) error {
	c.ChannelID = 0
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			var val uint64
			val, i = readVarintSafe(b, i)
			c.ChannelID = uint32(val)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- CryptSetup ---
// CryptSetup negotiates OCB-AES encryption for audio.
// The server sends it after Authenticate; the client replies with its nonce.
type CryptSetup struct {
	Key         []byte // field 1: 32 bytes AES key
	ClientNonce []byte // field 2: 24 bytes client nonce
	ServerNonce []byte // field 3: 24 bytes server nonce (from server only)
}

func (c *CryptSetup) Marshal() ([]byte, error) {
	var b []byte
	if len(c.Key) > 0 {
		b = append(b, 0x0a)
		b = appendVarint(b, uint64(len(c.Key)))
		b = append(b, c.Key...)
	}
	if len(c.ClientNonce) > 0 {
		b = append(b, 0x12)
		b = appendVarint(b, uint64(len(c.ClientNonce)))
		b = append(b, c.ClientNonce...)
	}
	if len(c.ServerNonce) > 0 {
		b = append(b, 0x1a)
		b = appendVarint(b, uint64(len(c.ServerNonce)))
		b = append(b, c.ServerNonce...)
	}
	return b, nil
}

func (c *CryptSetup) Unmarshal(b []byte) error {
	c.Key = nil
	c.ClientNonce = nil
	c.ServerNonce = nil
	i := 0
	for i < len(b) {
		tag := b[i]
		fieldNum := int(tag >> 3)
		wire := int(tag & 7)
		i++
		if i >= len(b) {
			break
		}
		switch fieldNum {
		case 1:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			c.Key = b[i : i+int(n)]
			i += int(n)
		case 2:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			c.ClientNonce = b[i : i+int(n)]
			i += int(n)
		case 3:
			n, ni := readVarintSafe(b, i)
			i = ni
			if n == 0 || i+int(n) > len(b) {
				break
			}
			c.ServerNonce = b[i : i+int(n)]
			i += int(n)
		default:
			i = skipEnd(b, i, wire)
		}
	}
	return nil
}

// --- Helpers ---

// Library version info. Mumble version = major<<16 | minor<<8 | patch
const (
	ProtoMajor     = 1
	ProtoMinor     = 5
	ProtoPatch     = 0
	LibraryVersion = ProtoMajor<<16 | ProtoMinor<<8 | ProtoPatch
	LibraryRelease = "mumble-go"
	LibraryOS      = "Go"
)

// MakeVersion returns a Version for this library.
func MakeVersion() *Version {
	return &Version{
		Version: LibraryVersion,
		Release: LibraryRelease,
		OS:      LibraryOS,
	}
}

// MakeAuthenticate returns an Authenticate message.
func MakeAuthenticate(username, password string, tokens []string, opus bool) *Authenticate {
	return &Authenticate{
		Username: username,
		Password: password,
		Tokens:   tokens,
		Opus:     opus,
	}
}

// ParseServerSync parses a ServerSync payload.
func ParseServerSync(b []byte) (*ServerSync, error) {
	m := &ServerSync{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseChannelState parses a ChannelState payload.
func ParseChannelState(b []byte) (*ChannelState, error) {
	m := &ChannelState{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseUserState parses a UserState payload.
func ParseUserState(b []byte) (*UserState, error) {
	m := &UserState{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseReject parses a Reject payload.
func ParseReject(b []byte) (*Reject, error) {
	m := &Reject{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParsePermissionDenied parses a PermissionDenied payload.
func ParsePermissionDenied(b []byte) (*PermissionDenied, error) {
	m := &PermissionDenied{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseCodecVersion parses a CodecVersion payload.
func ParseCodecVersion(b []byte) (*CodecVersion, error) {
	m := &CodecVersion{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseUserRemove parses a UserRemove payload.
func ParseUserRemove(b []byte) (*UserRemove, error) {
	m := &UserRemove{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseChannelRemove parses a ChannelRemove payload.
func ParseChannelRemove(b []byte) (*ChannelRemove, error) {
	m := &ChannelRemove{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseServerConfig parses a ServerConfig payload.
func ParseServerConfig(b []byte) (*ServerConfig, error) {
	m := &ServerConfig{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseTextMessage parses a TextMessage payload.
func ParseTextMessage(b []byte) (*TextMessage, error) {
	m := &TextMessage{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParseCryptSetup parses a CryptSetup payload.
func ParseCryptSetup(b []byte) (*CryptSetup, error) {
	m := &CryptSetup{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// ParsePing parses a Ping payload.
func ParsePing(b []byte) (*Ping, error) {
	m := &Ping{}
	if err := m.Unmarshal(b); err != nil {
		return nil, err
	}
	return m, nil
}

// --- Low-level protobuf helpers ---

// readVarintSafe reads a varint; returns (0, len(b)) if at end of buffer.
// Does NOT panic on truncation.
// If the remaining bytes form a truncated multi-byte varint (EOF while reading
// continuation bytes), returns (0, len(b)) so callers don't overrun.
func readVarintSafe(b []byte, i int) (uint64, int) {
	if i >= len(b) {
		return 0, len(b)
	}
	var val uint64
	var shift uint
	for i < len(b) {
		c := b[i]
		i++
		val |= uint64(c&0x7F) << shift
		if c&0x80 == 0 {
			return val, i // success: found terminating byte
		}
		shift += 7
		// Truncated varint: last byte had continuation bit set.
		// This is invalid in a complete protobuf message; treat as terminal.
		if i >= len(b) {
			return 0, len(b)
		}
	}
	return val, i
}

// skipEnd advances the index past the end of a field, given its wire type.
// Called after the tag has been consumed; i is the start of field data.
func skipEnd(b []byte, i int, wire int) int {
	switch wire {
	case wireVarint:
		_, i = readVarintSafe(b, i)
	case wireFixed64:
		i += 8
	case wireLengthDelim:
		_, i = readVarintSafe(b, i)
	case wireFixed32:
		i += 4
	}
	if i > len(b) {
		return len(b)
	}
	return i
}

// appendVarint encodes and appends a varint to b.
func appendVarint(b []byte, val uint64) []byte {
	for val >= 0x80 {
		b = append(b, byte(val&0x7F|0x80))
		val >>= 7
	}
	b = append(b, byte(val))
	return b
}

// appendFixed64 appends 8 bytes in little-endian order.
func appendFixed64(b []byte, bits uint64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], bits)
	return append(b, buf[:]...)
}
