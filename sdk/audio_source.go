package sdk

import (
	"context"
	"errors"
	"io"
)

// AudioSource produces PCM frames in SDK contract format:
// 48kHz, mono, signed 16-bit little-endian PCM.
type AudioSource interface {
	ReadPCM(ctx context.Context, dst []byte) (int, error)
}

// StreamPCM sends data from an AudioSource until EOF or context cancellation.
// It groups a few 20ms frames together before flushing to better match real
// client voice cadence for sustained playback.
func (c *Client) StreamPCM(ctx context.Context, src AudioSource, frameBytes int) error {
	if frameBytes <= 0 {
		return errors.New("sdk: frameBytes must be > 0")
	}
	const bundleFrames = 5 // 100ms packets
	buf := make([]byte, frameBytes)
	pending := make([]byte, 0, frameBytes*bundleFrames)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if err := c.SendPCM(pending); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}
	for {
		n, err := src.ReadPCM(ctx, buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			if len(pending) >= cap(pending) {
				if sendErr := flush(); sendErr != nil {
					return sendErr
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return flush()
			}
			return err
		}
	}
}
