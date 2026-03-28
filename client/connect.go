package client

import (
	"context"
	"log"
)

// Connect opens the transport, completes the handshake, and starts background goroutines.
func (c *Client) Connect(ctx context.Context) error {
	if err := c.dial(); err != nil {
		return err
	}
	if err := c.handshake(ctx); err != nil {
		c.conn.Close()
		return err
	}

	c.setupAudioTransport()
	if err := c.setupUDP(); err != nil {
		log.Printf("[client] UDP setup failed: %v (continuing with TCP only)", err)
	}

	c.events.emitConnect()
	newClientRuntime(c, ctx).start()
	return nil
}

func (c *Client) setupAudioTransport() {
	if c.audio == nil {
		return
	}

	c.audio.SetWriter(c.conn)
	c.audio.SetSession(c.store.SelfSession())
	if len(c.cryptoKey) == 16 && len(c.cryptoClientNonce) == 16 && len(c.cryptoServerNonce) >= 16 {
		c.audio.SetCrypto(c.cryptoKey, c.cryptoClientNonce, c.cryptoServerNonce[:16])
	}

	existingCb := c.audio.GetCallback()
	c.audio.SetCallback(func(session uint32, seq uint64, pcm []byte) {
		if existingCb != nil {
			existingCb(session, seq, pcm)
		}
		c.events.emitAudio(session, seq, pcm)
	})
}

func (c *Client) setupUDP() error {
	udpPort, err := c.SetupUDP()
	if err != nil {
		return err
	}
	log.Printf("[client] UDP audio initialized on port %d", udpPort)
	return nil
}
