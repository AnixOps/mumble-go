package sdk

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// AudioSource produces PCM frames in SDK contract format:
// 48kHz, mono, signed 16-bit little-endian PCM.
type AudioSource interface {
	ReadPCM(ctx context.Context, dst []byte) (int, error)
}

// StreamPCM sends data from an AudioSource until EOF or context cancellation.
// It sends frames in small chunks (~20ms each) to match Mumble protocol expectations.
func (c *Client) StreamPCM(ctx context.Context, src AudioSource, frameBytes int) error {
	if frameBytes <= 0 {
		return errors.New("sdk: frameBytes must be > 0")
	}

	buf := make([]byte, frameBytes) // 1920 bytes = 20ms at 48kHz

	for {
		n, err := src.ReadPCM(ctx, buf)
		if n > 0 {
			fmt.Printf("[DEBUG] StreamPCM: got %d bytes from source\n", n)
			if err := c.SendPCM(buf[:n]); err != nil {
				fmt.Printf("[DEBUG] StreamPCM: SendPCM error: %v\n", err)
				return err
			}
			fmt.Printf("[DEBUG] StreamPCM: sent %d bytes\n", n)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Printf("[DEBUG] StreamPCM: EOF\n")
				return nil
			}
			fmt.Printf("[DEBUG] StreamPCM: read error: %v\n", err)
			return err
		}
	}
}
