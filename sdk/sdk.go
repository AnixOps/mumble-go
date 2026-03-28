package sdk

import (
	"context"
	"crypto/tls"
	"fmt"

	"mumble-go/client"
	"mumble-go/identity"
)

type Config struct {
	Address         string
	Username        string
	Password        string
	IdentityProfile string
	IdentityDir     string
	InsecureTLS     bool
	Bandwidth       int
}

type AudioFrame struct {
	Session  uint32
	Sequence uint64
	PCM      []byte
}

type EventHandlers struct {
	OnConnect    func()
	OnDisconnect func()
	OnText       func(actor uint32, message string)
	OnAudio      func(frame AudioFrame)
	OnUserState  func(session uint32)
}

type Client struct {
	inner *client.Client
}

func New(cfg Config) *Client {
	audioCfg := client.DefaultAudioConfig()
	if cfg.Bandwidth > 0 {
		audioCfg.Bandwidth = cfg.Bandwidth
	}

	clientCfg := client.Config{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		TLS: &tls.Config{
			ServerName:         cfg.Address,
			InsecureSkipVerify: cfg.InsecureTLS,
		},
		Audio: audioCfg,
	}
	if cfg.IdentityProfile != "" {
		clientCfg.Identity = identity.NewLocalStore(cfg.IdentityDir, cfg.IdentityProfile)
	}

	return &Client{inner: client.New(clientCfg)}
}

func (c *Client) ConfigureOpus() error {
	return c.inner.Audio().SetOpusCodec()
}

func (c *Client) ConfigureRawAudio() error {
	return c.inner.Audio().SetRawCodec()
}

func (c *Client) SetHandlers(h EventHandlers) {
	if h.OnAudio != nil {
		c.inner.OnAudio(func(session uint32, seq uint64, pcm []byte) {
			h.OnAudio(AudioFrame{Session: session, Sequence: seq, PCM: pcm})
		})
	}
	events := c.inner.Events()
	if h.OnConnect != nil {
		events.OnConnect(h.OnConnect)
	}
	if h.OnDisconnect != nil {
		events.OnDisconnect(h.OnDisconnect)
	}
	if h.OnText != nil {
		events.OnTextMessage(h.OnText)
	}
	if h.OnUserState != nil {
		events.OnUserState(h.OnUserState)
	}
}

func (c *Client) Connect(ctx context.Context) error {
	return c.inner.Connect(ctx)
}

func (c *Client) SelfRegister() error {
	return c.inner.SelfRegister()
}

func (c *Client) JoinChannelByName(name string) error {
	return c.inner.JoinChannelByName(name)
}

func (c *Client) JoinChannel(id uint32) error {
	return c.inner.JoinChannel(id)
}

func (c *Client) SendPCM(pcm []byte) error {
	return c.inner.SendAudio(pcm)
}

func (c *Client) SendPCMUDP(pcm []byte) error {
	return c.inner.SendAudioUDP(pcm)
}

func (c *Client) Session() uint32 {
	return c.inner.State().SelfSession()
}

func (c *Client) RegisteredUserID() (uint32, bool) {
	session := c.inner.State().SelfSession()
	if session == 0 {
		return 0, false
	}
	user, ok := c.inner.State().SnapshotUsers()[session]
	if !ok {
		return 0, false
	}
	return user.UserID, true
}

func (c *Client) ChannelID() (uint32, bool) {
	session := c.inner.State().SelfSession()
	if session == 0 {
		return 0, false
	}
	user, ok := c.inner.State().SnapshotUsers()[session]
	if !ok {
		return 0, false
	}
	return user.ChannelID, user.HasChannelID
}

func (c *Client) IdentityState() string {
	session := c.inner.State().SelfSession()
	if session == 0 {
		return "disconnected"
	}
	user, ok := c.inner.State().SnapshotUsers()[session]
	if !ok {
		return "connected"
	}
	return fmt.Sprintf("session=%d user_id=%d channel=%d cert_hash=%s", user.Session, user.UserID, user.ChannelID, user.CertificateHash)
}

func (c *Client) Close() error {
	return c.inner.Close()
}
