// testbot is a test Mumble bot that can send and receive audio.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sync/atomic"
	"time"

	"mumble-go/client"
	"mumble-go/identity"
	"mumble-go/protocol"
)

var (
	addr         = flag.String("addr", "mumble.hotxiang.cn:64738", "server address")
	username     = flag.String("user", "testbot", "username")
	password     = flag.String("pass", "", "password")
	channel      = flag.String("channel", "", "channel to join")
	raw          = flag.Bool("raw", false, "use raw codec instead of Opus (for testing without libopus)")
	sendAudio    = flag.Bool("send-audio", false, "continuously send audio")
	sendMessage  = flag.String("send-msg", "", "send a text message")
	mute         = flag.Bool("mute", false, "set self-mute")
	deaf         = flag.Bool("deaf", false, "set self-deaf")
	debug        = flag.Bool("debug", false, "enable debug logging")
	identityProfile = flag.String("identity-profile", "", "persistent client identity profile")
	identityDir  = flag.String("identity-dir", "", "directory for persisted client identities")
	selfRegister = flag.Bool("self-register", false, "attempt self-registration after connecting")
)

func main() {
	flag.Parse()

	if *debug {
		protocol.EnableDebug = true
	}

	// Audio callback - print when audio received
	var audioReceived atomic.Int32

	cfg := client.Config{
		Address:  *addr,
		Username: *username,
		Password: *password,
		TLS: &tls.Config{
			ServerName:         "mumble.hotxiang.cn",
			InsecureSkipVerify: true,
		},
		Audio: client.DefaultAudioConfig(),
	}
	if *identityProfile != "" {
		cfg.Identity = identity.NewLocalStore(*identityDir, *identityProfile)
		if meta, err := identity.NewLocalStore(*identityDir, *identityProfile).Metadata(); err == nil {
			fmt.Printf("Identity profile %s\n  sha256=%s\n  sha1=%s\n  subject=%s\n", *identityProfile, meta.Fingerprint, meta.CertificateSHA1, meta.Subject)
		}
	}

	c := client.New(cfg)

	// Set up event handlers
	events := c.Events()
	events.OnConnect(func() {
		fmt.Println("[EVENT] Connected to server")
	})
	events.OnDisconnect(func() {
		fmt.Println("[EVENT] Disconnected from server")
	})
	events.OnUserJoined(func(session uint32) {
		fmt.Printf("[EVENT] User joined: session=%d\n", session)
	})
	events.OnUserLeft(func(session uint32) {
		fmt.Printf("[EVENT] User left: session=%d\n", session)
	})
	events.OnUserState(func(session uint32) {
		users := c.State().SnapshotUsers()
		if u, ok := users[session]; ok {
			fmt.Printf("[EVENT] User state: session=%d name=%s user_id=%d channel=%d cert_hash=%s\n", session, u.Name, u.UserID, u.ChannelID, u.CertificateHash)
		}
	})
	events.OnChannelAdded(func(channelID uint32) {
		channels := c.State().SnapshotChannels()
		if ch, ok := channels[channelID]; ok {
			fmt.Printf("[EVENT] Channel added: id=%d name=%s\n", channelID, ch.Name)
		}
	})
	events.OnTextMessage(func(actor uint32, message string) {
		users := c.State().SnapshotUsers()
		name := "unknown"
		if u, ok := users[actor]; ok {
			name = u.Name
		}
		fmt.Printf("[EVENT] Text message from %s (session=%d): %s\n", name, actor, message)
	})

	// Set audio callback
	c.OnAudio(func(session uint32, seq uint64, pcm []byte) {
		audioReceived.Add(1)
		if *debug {
			users := c.State().SnapshotUsers()
			name := "unknown"
			if u, ok := users[session]; ok {
				name = u.Name
			}
			log.Printf("[audio] from %s (session=%d) seq=%d len=%d", name, session, seq, len(pcm))
		}
	})
	events.OnAudio(func(session uint32, seq uint64, pcm []byte) {
		// Audio events are also emitted for audio callback
	})

	// Set Opus codec by default, fall back to raw if opus fails
	if *raw {
		if err := c.Audio().SetRawCodec(); err != nil {
			fmt.Printf("FAIL: failed to set raw codec: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Using raw codec (testing mode)")
	} else {
		if err := c.Audio().SetOpusCodec(); err != nil {
			fmt.Printf("Warning: failed to set Opus codec: %v, falling back to raw\n", err)
			if err2 := c.Audio().SetRawCodec(); err2 != nil {
				fmt.Printf("FAIL: failed to set raw codec: %v\n", err2)
				os.Exit(1)
			}
			fmt.Println("Using raw codec (Opus failed)")
		} else {
			fmt.Println("Using Opus codec")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Connecting to %s as %s...\n", *addr, *username)
	if err := c.Connect(ctx); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("OK! session=%d opus=%v\n", c.State().SelfSession(), c.SupportsOpus())
	if self, ok := c.State().SnapshotUsers()[c.State().SelfSession()]; ok {
		fmt.Printf("Identity state: user_id=%d cert_hash=%s\n", self.UserID, self.CertificateHash)
	}

	// Wait for state to stabilize
	time.Sleep(1 * time.Second)

	if *selfRegister {
		fmt.Println("Attempting self-register...")
		if err := c.SelfRegister(); err != nil {
			fmt.Printf("Self-register failed: %v\n", err)
		} else {
			fmt.Println("Self-register request sent")
			time.Sleep(500 * time.Millisecond)
			if self, ok := c.State().SnapshotUsers()[c.State().SelfSession()]; ok {
				fmt.Printf("Post-register state: user_id=%d cert_hash=%s\n", self.UserID, self.CertificateHash)
			}
		}
	}

	// Join target channel if specified
	if *channel != "" {
		// Find channel ID by name
		channels := c.State().SnapshotChannels()
		var targetID uint32
		for id, ch := range channels {
			if ch.Name == *channel {
				targetID = id
				break
			}
		}
		if targetID == 0 {
			fmt.Printf("Channel '%s' not found\n", *channel)
		} else {
			fmt.Printf("Moving to channel %d (%s)...\n", targetID, *channel)
			if err := c.JoinChannel(targetID); err != nil {
				fmt.Printf("Failed to join channel: %v\n", err)
			} else {
				fmt.Printf("Joined channel %s successfully\n", *channel)
				time.Sleep(500 * time.Millisecond) // Wait for server to update
			}
		}
	}

	// Print channel/user info
	channels := c.State().SnapshotChannels()
	fmt.Printf("\nChannels (%d):\n", len(channels))
	for id, ch := range channels {
		fmt.Printf("  [%2d] %s\n", id, ch.Name)
	}

	users := c.State().SnapshotUsers()
	fmt.Printf("\nUsers (%d):\n", len(users))
	for id, u := range users {
		fmt.Printf("  [%2d] %s (channel=%d user_id=%d cert_hash=%s", id, u.Name, u.ChannelID, u.UserID, u.CertificateHash)
		if u.Muted {
			fmt.Print(" muted")
		}
		if u.Deafened {
			fmt.Print(" deaf")
		}
		fmt.Println(")")
	}

	// Handle mute/deaf
	if *mute {
		fmt.Println("Setting self-mute...")
		if err := c.SetSelfMute(true); err != nil {
			fmt.Printf("Failed to set self-mute: %v\n", err)
		} else {
			fmt.Println("Self-mute set")
		}
	}

	if *deaf {
		fmt.Println("Setting self-deaf...")
		if err := c.SetSelfDeaf(true); err != nil {
			fmt.Printf("Failed to set self-deaf: %v\n", err)
		} else {
			fmt.Println("Self-deaf set")
		}
	}

	// Send message if requested
	if *sendMessage != "" {
		fmt.Printf("Sending message: %s\n", *sendMessage)
		// Send to all users in the same channel
		selfSession := c.State().SelfSession()
		selfChannel := c.State().SnapshotUsers()[selfSession].ChannelID
		var sessions []uint32
		for id, u := range c.State().SnapshotUsers() {
			if u.ChannelID == selfChannel && id != selfSession {
				sessions = append(sessions, id)
			}
		}
		if err := c.SendMessage(*sendMessage, sessions); err != nil {
			fmt.Printf("Failed to send message: %v\n", err)
		} else {
			fmt.Println("Message sent")
		}
	}

	// Keep running
	fmt.Println("\nBot is running. Press Ctrl+C to exit.")
	fmt.Printf("Audio received count: %d\n", audioReceived.Load())

	// Audio sending ticker (if enabled)
	var audioTicker *time.Ticker
	var audioTick <-chan time.Time
	if *sendAudio {
		audioTicker = time.NewTicker(2 * time.Second)
		audioTick = audioTicker.C
		fmt.Println("Audio sending enabled - will send tone every 2 seconds")
	}

	// Periodic status
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			if audioTicker != nil {
				audioTicker.Stop()
			}
			c.Close()
			return
		case <-audioTick:
			// Send a test tone (440Hz, 500ms)
			tone := generateTone(440, 500, 48000, 0.3)
			if err := c.SendAudio(tone); err != nil {
				fmt.Printf("Audio send error: %v\n", err)
			} else {
				fmt.Printf("[%s] Sent audio tone (440Hz, 500ms)\n", time.Now().Format("15:04:05"))
			}
		case <-statusTicker.C:
			count := audioReceived.Load()
			fmt.Printf("[%s] Audio received: %d\n", time.Now().Format("15:04:05"), count)
		}
	}
}


// generateTone generates a PCM sine wave tone.
func generateTone(frequency, durationMs, sampleRate int, amplitude float64) []byte {
	frameSize := 960 // 20ms at 48kHz
	frames := (durationMs * sampleRate / 1000) / frameSize
	if frames == 0 {
		frames = 1
	}
	samples := frames * frameSize

	pcm := make([]byte, samples*2) // 16-bit samples

	for i := 0; i < samples; i++ {
		sample := int16(amplitude * 32767 * math.Sin(2*math.Pi*float64(frequency)*float64(i)/float64(sampleRate)))
		pcm[i*2] = byte(sample)
		pcm[i*2+1] = byte(sample >> 8)
	}

	return pcm
}
