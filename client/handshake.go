package client

import (
	"context"
	"fmt"
	"log"

	"mumble-go/protocol"
	"mumble-go/state"
)

func (c *Client) handshake(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("client: not connected")
	}

	// 1. Send Version
	v := protocol.MakeVersion()
	vPayload, err := v.Marshal()
	if err != nil {
		return fmt.Errorf("version marshal: %w", err)
	}
	if err := c.conn.WriteFrame(protocol.MessageTypeVersion, vPayload); err != nil {
		return err
	}

	// 2. Send Authenticate
	auth := protocol.MakeAuthenticate(
		c.cfg.Username,
		c.cfg.Password,
		c.cfg.Tokens,
		true, // opus
	)
	authPayload, err := auth.Marshal()
	if err != nil {
		return fmt.Errorf("auth marshal: %w", err)
	}
	if err := c.conn.WriteFrame(protocol.MessageTypeAuthenticate, authPayload); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t, payload, err := c.conn.ReadFrame()
		if err != nil {
			return err
		}

		switch t {
		case protocol.MessageTypeServerSync:
			ss, err := protocol.ParseServerSync(payload)
			if err != nil {
				return fmt.Errorf("serverSync: %w", err)
			}
			if ss.Session != 0 {
				self := state.User{Session: uint32(ss.Session)}
				c.store.SetSelf(self)
			}
			c.store.MarkReady()
			return nil

		case protocol.MessageTypeCryptSetup:
			cs, err := protocol.ParseCryptSetup(payload)
			if err != nil {
				if protocol.EnableDebug {
					log.Printf("[handshake] CryptSetup parse error: %v", err)
				}
				continue
			}
			// Store crypto keys for audio
			c.cryptoKey = cs.Key
			c.cryptoServerNonce = cs.ServerNonce
			// Client must respond with its own nonce
			if len(cs.Key) >= 16 && len(cs.ServerNonce) >= 16 {
				clientNonce, err := generateNonce()
				if err != nil {
					log.Printf("[handshake] failed to generate nonce: %v", err)
					continue
				}
				c.cryptoClientNonce = clientNonce[:16] // Use first 16 bytes for IV
				reply := &protocol.CryptSetup{
					Key:         cs.Key,
					ClientNonce: clientNonce,
				}
				replyPayload, err := reply.Marshal()
				if err == nil {
					c.conn.WriteFrame(protocol.MessageTypeCryptSetup, replyPayload)
				}
			}

		case protocol.MessageTypeChannelState:
			cs, err := protocol.ParseChannelState(payload)
			if err != nil {
				if protocol.EnableDebug {
					log.Printf("[handshake] ChannelState parse error: %v", err)
				}
				continue
			}
			if cs.HasChannelID {
				c.store.UpsertChannelFromProto(cs)
			}

		case protocol.MessageTypeUserState:
			us, err := protocol.ParseUserState(payload)
			if err != nil {
				if protocol.EnableDebug {
					log.Printf("[handshake] UserState parse error: %v", err)
				}
				continue
			}
			c.store.UpsertUserFromProto(us)

		case protocol.MessageTypeUserRemove:
			ur, err := protocol.ParseUserRemove(payload)
			if err == nil && ur.Session != 0 {
				c.store.RemoveUser(ur.Session)
			}

		case protocol.MessageTypeTextMessage:
			// Silently handle for now; stored for future event API
			_, _ = protocol.ParseTextMessage(payload)

		case protocol.MessageTypeCodecVersion:
			cv, err := protocol.ParseCodecVersion(payload)
			if err != nil {
				if protocol.EnableDebug {
					log.Printf("[handshake] CodecVersion parse error: %v", err)
				}
				continue
			}
			c.codecOpus = cv.Opus
			if protocol.EnableDebug {
				log.Printf("[handshake] CodecVersion: opus=%v opusSupport=%v", cv.Opus, cv.OpusSupport)
			}

		case protocol.MessageTypeReject:
			rj, err := protocol.ParseReject(payload)
			if err != nil {
				return fmt.Errorf("client: connection rejected (parse error)")
			}
			reason := "unknown"
			if rj.Reason != "" {
				reason = rj.Reason
			}
			return fmt.Errorf("client: rejected: %s", reason)

		case protocol.MessageTypePing:
			c.handlePing(payload)

		default:
			if protocol.EnableDebug {
				log.Printf("[handshake] unhandled message type=%d len=%d", t, len(payload))
			}
		}
	}
}

// handlePing responds to a Ping message with a Pong.
func (c *Client) handlePing(payload []byte) {
	ping, err := protocol.ParsePing(payload)
	if err != nil {
		if protocol.EnableDebug {
			log.Printf("[handshake] ping parse error: %v", err)
		}
		return
	}
	// Respond with a Ping containing the same timestamp
	pong := &protocol.Ping{
		Timestamp: ping.Timestamp,
		NumPackets: 0,
		NumBytes:   0,
	}
	pongPayload, err := pong.Marshal()
	if err != nil {
		return
	}
	c.conn.WriteFrame(protocol.MessageTypePing, pongPayload)
}
