package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"mumble-go/sdk"
)

var (
	addr = flag.String("addr", "145.239.90.226:64738", "server address")
	user = flag.String("user", "audio-bot", "bot username")
	pass = flag.String("pass", "", "password")
	channel = flag.String("channel", "test", "channel name")
	identityProfile = flag.String("identity-profile", "", "persistent identity profile")
	identityDir = flag.String("identity-dir", "", "identity directory")
	wav = flag.String("wav", "", "path to 48kHz mono 16-bit PCM WAV file")
	pcm = flag.String("pcm", "", "path to raw 48kHz mono 16-bit PCM file")
	file = flag.String("file", "", "path or URL for ffmpeg-decoded audio input")
	soundcloud = flag.String("soundcloud", "", "SoundCloud or other supported page URL resolved via yt-dlp")
	selfRegister = flag.Bool("self-register", false, "attempt self registration")
)

func main() {
	flag.Parse()
	if *wav == "" && *pcm == "" && *file == "" && *soundcloud == "" {
		log.Fatal("provide -wav, -pcm, -file, or -soundcloud")
	}

	c := sdk.New(sdk.Config{
		Address:         *addr,
		Username:        *user,
		Password:        *pass,
		IdentityProfile: *identityProfile,
		IdentityDir:     *identityDir,
		InsecureTLS:     true,
	})
	defer c.Close()

	if err := c.ConfigureRawAudio(); err != nil {
		log.Fatalf("configure audio: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}
	if *selfRegister {
		if err := c.SelfRegister(); err != nil {
			log.Printf("self-register: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if *channel != "" {
		if err := c.JoinChannelByName(*channel); err != nil {
			log.Printf("join channel: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println(c.IdentityState())

	frameBytes := 960 * 2
	if *wav != "" {
		src, err := sdk.NewWAVSource(*wav)
		if err != nil {
			log.Fatalf("open wav: %v", err)
		}
		defer src.Close()
		if err := c.StreamPCM(context.Background(), src, frameBytes); err != nil {
			log.Fatalf("stream wav: %v", err)
		}
		return
	}
	if *soundcloud != "" {
		if err := c.PlayRemote(context.Background(), *soundcloud); err != nil {
			log.Fatalf("play remote: %v", err)
		}
		return
	}
	if *file != "" {
		if err := c.PlayFile(context.Background(), *file); err != nil {
			log.Fatalf("play file: %v", err)
		}
		return
	}

	src, err := sdk.NewFileSource(*pcm)
	if err != nil {
		log.Fatalf("open pcm: %v", err)
	}
	defer src.Close()
	if err := c.StreamPCM(context.Background(), src, frameBytes); err != nil {
		log.Fatalf("stream pcm: %v", err)
	}
}
