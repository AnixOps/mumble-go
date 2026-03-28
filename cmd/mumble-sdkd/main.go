package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mumble-go/sdk"
)

type request struct {
	Address         string `json:"address"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	IdentityProfile string `json:"identity_profile"`
	IdentityDir     string `json:"identity_dir"`
	InsecureTLS     bool   `json:"insecure_tls"`
	Channel         string `json:"channel"`
	SelfRegister    bool   `json:"self_register"`
	PlayFile        string `json:"play_file"`
	PlayWAV         string `json:"play_wav"`
	PlayPCM         string `json:"play_pcm"`
	PlayRemote      string `json:"play_remote"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "decode request: %v\n", err)
		os.Exit(1)
	}

	c := sdk.New(sdk.Config{
		Address:         req.Address,
		Username:        req.Username,
		Password:        req.Password,
		IdentityProfile: req.IdentityProfile,
		IdentityDir:     req.IdentityDir,
		InsecureTLS:     req.InsecureTLS,
	})
	defer c.Close()
	_ = c.ConfigureRawAudio()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	if req.SelfRegister {
		_ = c.SelfRegister()
		time.Sleep(500 * time.Millisecond)
	}
	if req.Channel != "" {
		_ = c.JoinChannelByName(req.Channel)
		time.Sleep(500 * time.Millisecond)
	}
	if req.PlayRemote != "" {
		playCtx, playCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer playCancel()
		if err := c.PlayRemote(playCtx, req.PlayRemote); err != nil {
			fmt.Fprintf(os.Stderr, "play_remote: %v\n", err)
			os.Exit(1)
		}
	}
	if req.PlayWAV != "" {
		src, err := sdk.NewWAVSource(req.PlayWAV)
		if err != nil {
			fmt.Fprintf(os.Stderr, "play_wav: %v\n", err)
			os.Exit(1)
		}
		defer src.Close()
		if err := c.StreamPCM(context.Background(), src, 960*2); err != nil {
			fmt.Fprintf(os.Stderr, "play_wav: %v\n", err)
			os.Exit(1)
		}
	}
	if req.PlayPCM != "" {
		src, err := sdk.NewFileSource(req.PlayPCM)
		if err != nil {
			fmt.Fprintf(os.Stderr, "play_pcm: %v\n", err)
			os.Exit(1)
		}
		defer src.Close()
		if err := c.StreamPCM(context.Background(), src, 960*2); err != nil {
			fmt.Fprintf(os.Stderr, "play_pcm: %v\n", err)
			os.Exit(1)
		}
	}
	if req.PlayFile != "" {
		playCtx, playCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer playCancel()
		if err := c.PlayFile(playCtx, req.PlayFile); err != nil {
			fmt.Fprintf(os.Stderr, "play_file: %v\n", err)
			os.Exit(1)
		}
	}

	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"session":        c.Session(),
		"identity_state": c.IdentityState(),
	})
}
