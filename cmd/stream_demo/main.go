// StreamDemo demonstrates the StreamSender API for commercial-grade audio streaming.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mumble-go/sdk"
	"mumble-go/sdk/stream"
)

var (
	addr            = flag.String("addr", "145.239.90.226:64738", "server address")
	user            = flag.String("user", "stream-bot", "bot username")
	pass            = flag.String("pass", "", "password")
	channel         = flag.String("channel", "test", "channel name")
	identityProfile = flag.String("identity-profile", "", "persistent identity profile")
	identityDir     = flag.String("identity-dir", "", "identity directory")
	wav             = flag.String("wav", "", "path to 48kHz mono 16-bit PCM WAV file")
	file            = flag.String("file", "", "path or URL for ffmpeg-decoded audio input")
)

func main() {
	flag.Parse()
	if *wav == "" && *file == "" {
		log.Fatal("provide -wav or -file")
	}

	// Create SDK client
	c := sdk.New(sdk.Config{
		Address:         *addr,
		Username:        *user,
		Password:        *pass,
		IdentityProfile: *identityProfile,
		IdentityDir:     *identityDir,
		InsecureTLS:     true,
	})
	defer c.Close()

	// Configure Opus codec for production audio
	if err := c.ConfigureOpus(); err != nil {
		log.Fatalf("configure opus: %v", err)
	}

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}

	// Join channel
	if *channel != "" {
		if err := c.JoinChannelByName(*channel); err != nil {
			log.Printf("join channel: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Connected:", c.IdentityState())

	// Create stream sender with production config
	cfg := stream.DefaultStreamConfig()
	cfg.Bitrate = 64000
	cfg.VADEnabled = true
	cfg.EnergyThreshold = 500
	cfg.BufferDepth = 3
	cfg.Reconnect = true
	cfg.MaxAttempts = 10

	sender, err := stream.NewStreamSender(c, cfg)
	if err != nil {
		log.Fatalf("create stream sender: %v", err)
	}

	// Set up event handlers
	sender.Events().OnConnect = func() {
		fmt.Println("[stream] connected")
	}
	sender.Events().OnDisconnect = func(err error) {
		fmt.Println("[stream] disconnected:", err)
	}
	sender.Events().OnReconnecting = func(attempt int, nextDelay time.Duration) {
		fmt.Printf("[stream] reconnecting attempt %d, next retry in %v\n", attempt, nextDelay)
	}
	sender.Events().OnVADChange = func(speaking bool) {
		fmt.Println("[stream] VAD:", speaking)
	}
	sender.Events().OnError = func(err error) {
		fmt.Println("[stream] error:", err)
	}

	// Create audio source
	var src stream.AudioSource
	if *wav != "" {
		src, err = sdk.NewWAVSource(*wav)
	} else {
		src, err = sdk.NewFFmpegSource(*file)
	}
	if err != nil {
		log.Fatalf("create audio source: %v", err)
	}
	if closer, ok := src.(interface{ Close() }); ok {
		defer closer.Close()
	}

	// Set source and start streaming
	if err := sender.SetSource(src); err != nil {
		log.Fatalf("set source: %v", err)
	}

	// Set metadata (optional)
	sender.SetMetadata(&stream.StreamMetadata{
		Title:  "Streaming Demo",
		Artist: "StreamBot",
	})

	// Start streaming (non-blocking)
	streamCtx, streamCancel := context.WithCancel(context.Background())
	defer streamCancel()

	if err := sender.Start(streamCtx); err != nil {
		log.Fatalf("start stream: %v", err)
	}
	fmt.Println("[stream] started streaming")

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("[stream] stopping...")
	sender.Stop()
	fmt.Println("[stream] stopped")
}
