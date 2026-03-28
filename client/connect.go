package client

import (
	"context"
	"log"
	"time"

	"mumble-go/protocol"
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

	// Initialize audio writer
	if c.audio != nil {
		c.audio.SetWriter(c.conn)
		// Set our session for outgoing audio
		c.audio.SetSession(c.store.SelfSession())
		// Setup crypto if available
		if len(c.cryptoKey) == 16 && len(c.cryptoClientNonce) == 16 && len(c.cryptoServerNonce) >= 16 {
			c.audio.SetCrypto(c.cryptoKey, c.cryptoClientNonce, c.cryptoServerNonce[:16])
		}
		// Wrap existing audio callback to also emit events
		existingCb := c.audio.GetCallback()
		c.audio.SetCallback(func(session uint32, seq uint64, pcm []byte) {
			if existingCb != nil {
				existingCb(session, seq, pcm)
			}
			c.events.emitAudio(session, seq, pcm)
		})
	}

	// Initialize UDP audio manager
	udpPort, err := c.SetupUDP()
	if err != nil {
		log.Printf("[client] UDP setup failed: %v (continuing with TCP only)", err)
	} else {
		log.Printf("[client] UDP audio initialized on port %d", udpPort)
	}

	// Emit connect event
	c.events.emitConnect()

	// Start background message handler and ping loop
	c.wg.Add(2)
	go c.messageLoop()
	go c.pingLoop()
	return nil
}

// messageLoop handles incoming control messages after handshake.
func (c *Client) messageLoop() {
	defer c.wg.Done()
	defer c.events.emitDisconnect()
	for {
		select {
		case <-c.closeCh:
			return
		default:
		}
		t, payload, err := c.conn.ReadFrame()
		if err != nil {
			select {
			case <-c.closeCh:
				return
			default:
				log.Printf("[client] connection error: %v", err)
				return
			}
		}
		c.handleMessage(t, payload)
	}
}

// handleMessage processes a single control message.
func (c *Client) handleMessage(t protocol.MessageType, payload []byte) {
	switch t {
	case protocol.MessageTypeUDPTunnel:
		c.handleUDPTunnel(payload)
	case protocol.MessageTypePing:
		c.handlePing(payload)
	case protocol.MessageTypeChannelState:
		cs, err := protocol.ParseChannelState(payload)
		if err != nil {
			if protocol.EnableDebug {
				log.Printf("[client] ChannelState parse error: %v", err)
			}
			return
		}
		if cs.HasChannelID {
			c.store.UpsertChannelFromProto(cs)
			c.events.emitChannelAdded(cs.ChannelID)
		}
	case protocol.MessageTypeUserState:
		us, err := protocol.ParseUserState(payload)
		if err != nil {
			if protocol.EnableDebug {
				log.Printf("[client] UserState parse error: %v", err)
			}
			return
		}
		c.store.UpsertUserFromProto(us)
		if us.Session != 0 {
			c.events.emitUserState(us.Session)
		}
	case protocol.MessageTypeUserRemove:
		ur, err := protocol.ParseUserRemove(payload)
		if err == nil && ur.Session != 0 {
			c.store.RemoveUser(ur.Session)
			c.events.emitUserLeft(ur.Session)
		}
	case protocol.MessageTypeTextMessage:
		tm, err := protocol.ParseTextMessage(payload)
		if err != nil {
			if protocol.EnableDebug {
				log.Printf("[client] TextMessage parse error: %v", err)
			}
			return
		}
		c.handleTextMessage(tm)
	case protocol.MessageTypeChannelRemove:
		cr, err := protocol.ParseChannelRemove(payload)
		if err == nil && cr.ChannelID != 0 {
			c.store.RemoveChannel(cr.ChannelID)
			c.events.emitChannelRemoved(cr.ChannelID)
		}
	case protocol.MessageTypeCryptSetup:
		cs, err := protocol.ParseCryptSetup(payload)
		if err != nil {
			if protocol.EnableDebug {
				log.Printf("[client] CryptSetup parse error: %v", err)
			}
			return
		}
		c.handleCryptSetup(cs)
	case protocol.MessageTypePermissionDenied:
		if pd, err := protocol.ParsePermissionDenied(payload); err == nil {
			log.Printf("[client] PermissionDenied: %s", pd.String())
		} else if protocol.EnableDebug {
			log.Printf("[client] PermissionDenied parse error: %v payload=% x", err, payload)
		}
	case protocol.MessageTypeServerConfig:
		if protocol.EnableDebug {
			if sc, err := protocol.ParseServerConfig(payload); err == nil {
				log.Printf("[client] ServerConfig: max_bandwidth=%d welcome='%s' html=%v msg_len=%d img_len=%d",
					sc.MaxBandwidth, sc.WelcomeText, sc.AllowHTML, sc.MessageLength, sc.ImageMessageLength)
			}
		}
	default:
		if protocol.EnableDebug {
			log.Printf("[client] unhandled message type=%d len=%d", t, len(payload))
		}
	}
}

// handleTextMessage processes an incoming text message.
func (c *Client) handleTextMessage(tm *protocol.TextMessage) {
	if protocol.EnableDebug {
		log.Printf("[client] text message from actor=%d: %s", tm.Actor, tm.Message)
	}
	c.events.emitTextMessage(tm.Actor, tm.Message)
}

// handleUDPTunnel processes an incoming UDP tunnel audio packet.
func (c *Client) handleUDPTunnel(payload []byte) {
	if protocol.EnableDebug {
		log.Printf("[client] UDPTunnel received: len=%d", len(payload))
	}
	if c.audio == nil {
		if protocol.EnableDebug {
			log.Printf("[client] audio is nil, dropping packet")
		}
		return
	}
	if err := c.audio.ProcessPacket(payload); err != nil {
		if protocol.EnableDebug {
			log.Printf("[client] audio packet error: %v", err)
		}
	}
}

// handleCryptSetup processes a CryptSetup message.
func (c *Client) handleCryptSetup(cs *protocol.CryptSetup) {
	if len(cs.Key) >= 16 && len(cs.ServerNonce) >= 16 {
		c.cryptoKey = cs.Key
		c.cryptoServerNonce = cs.ServerNonce
		// Respond with client nonce
		clientNonce, err := generateNonce()
		if err != nil {
			log.Printf("[client] failed to generate nonce: %v", err)
			return
		}
		c.cryptoClientNonce = clientNonce[:16]
		reply := &protocol.CryptSetup{
			Key:         cs.Key,
			ClientNonce: clientNonce,
		}
		replyPayload, err := reply.Marshal()
		if err == nil {
			c.conn.WriteFrame(protocol.MessageTypeCryptSetup, replyPayload)
		}
		// Update audio crypto
		if c.audio != nil && len(c.cryptoKey) >= 16 {
			c.audio.SetCrypto(c.cryptoKey[:16], c.cryptoClientNonce, c.cryptoServerNonce[:16])
		}
	}
}

// pingLoop sends periodic ping messages to the server.
func (c *Client) pingLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.closeCh:
			return
		case <-ticker.C:
			ping := &protocol.Ping{
				Timestamp: uint64(time.Now().Unix()),
			}
			payload, err := ping.Marshal()
			if err != nil {
				continue
			}
			if err := c.conn.WriteFrame(protocol.MessageTypePing, payload); err != nil {
				log.Printf("[client] ping failed: %v", err)
				return
			}
		}
	}
}
