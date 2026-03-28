package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"mumble-go/sdk"
)

func main() {
	client := sdk.New(sdk.Config{
		Address:     "145.239.90.226:64738",
		Username:    "sdk-demo",
		InsecureTLS: true,
	})
	defer client.Close()

	if err := client.ConfigureRawAudio(); err != nil {
		log.Fatalf("configure audio: %v", err)
	}

	client.SetHandlers(sdk.EventHandlers{
		OnConnect: func() { fmt.Println("connected") },
		OnAudio: func(frame sdk.AudioFrame) {
			fmt.Printf("audio session=%d seq=%d bytes=%d\n", frame.Session, frame.Sequence, len(frame.PCM))
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("connect: %v", err)
	}

	fmt.Println(client.IdentityState())
}
