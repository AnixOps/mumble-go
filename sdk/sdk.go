package sdk

import (
	"context"
	"crypto/tls"
	"fmt"
	"os/exec"

	"mumble-go/client"
	"mumble-go/identity"
	"mumble-go/state"
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

// SendAudio is an alias for SendPCM for API compatibility.
func (c *Client) SendAudio(pcm []byte) error {
	return c.inner.SendAudio(pcm)
}

// SendAudioUDP is an alias for SendPCMUDP for API compatibility.
func (c *Client) SendAudioUDP(pcm []byte) error {
	return c.inner.SendAudioUDP(pcm)
}

// Audio returns the underlying audio handler for advanced use.
func (c *Client) Audio() *client.Audio {
	return c.inner.Audio()
}

// State returns the underlying state store.
func (c *Client) State() *state.Store {
	return c.inner.State()
}

// Events returns the underlying event handler.
func (c *Client) Events() *client.EventHandler {
	return c.inner.Events()
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

func (c *Client) PlayFile(ctx context.Context, path string) error {
	src, err := NewFFmpegSource(path)
	if err != nil {
		return err
	}
	defer src.Close()
	return c.StreamPCM(ctx, src, 960*2)
}

func (c *Client) PlayRemote(ctx context.Context, input string) error {
	resolved, err := ResolvePlayableURL(ctx, input)
	if err != nil {
		return err
	}
	// For HLS streams (m3u8), use streaming mode with yt-dlp pipe
	if len(resolved) > 5 && resolved[len(resolved)-5:] == ".m3u8" {
		return c.playHLSStream(ctx, resolved)
	}
	return c.PlayFile(ctx, resolved)
}

func (c *Client) playHLSStream(ctx context.Context, m3u8URL string) error {
	// Use yt-dlp to pipe the stream through ffmpeg
	cmd := exec.CommandContext(ctx, resolveTool("yt-dlp"),
		"--no-playlist",
		"-o", "-",
		"-f", "bestaudio",
		"--",
		m3u8URL,
	)

	ffmpegCmd := exec.CommandContext(ctx, resolveTool("ffmpeg"),
		"-nostdin",
		"-loglevel", "warning",
		"-i", "pipe:0",
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "1",
		"pipe:1",
	)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	ffmpegCmd.Stdin = pipe

	stdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		pipe.Close()
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		pipe.Close()
		return fmt.Errorf("start yt-dlp: %w", err)
	}
	if err := ffmpegCmd.Start(); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		pipe.Close()
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	// Stream using our own loop
	buf := make([]byte, 960*2)
	for {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			ffmpegCmd.Process.Kill()
			cmd.Wait()
			ffmpegCmd.Wait()
			return ctx.Err()
		default:
		}
		n, err := stdout.Read(buf)
		if n > 0 {
			if err := c.SendPCM(buf[:n]); err != nil {
				cmd.Process.Kill()
				ffmpegCmd.Process.Kill()
				cmd.Wait()
				ffmpegCmd.Wait()
				return err
			}
		}
		if err != nil {
			cmd.Process.Kill()
			ffmpegCmd.Process.Kill()
			cmd.Wait()
			ffmpegCmd.Wait()
			return nil // EOF or error
		}
	}
}

func (c *Client) Close() error {
	return c.inner.Close()
}
