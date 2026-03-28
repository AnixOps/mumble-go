package client

import (
	"context"
	"log"
	"time"

	"mumble-go/protocol"
)

type clientRuntime struct {
	client *Client
	ctx    context.Context
}

func newClientRuntime(c *Client, ctx context.Context) *clientRuntime {
	return &clientRuntime{client: c, ctx: ctx}
}

func (r *clientRuntime) start() {
	r.client.wg.Add(2)
	go r.messageLoop()
	go r.pingLoop()
}

func (r *clientRuntime) messageLoop() {
	c := r.client
	defer c.wg.Done()
	defer c.events.emitDisconnect()
	for {
		select {
		case <-c.closeCh:
			return
		case <-r.ctx.Done():
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
		r.handleMessage(t, payload)
	}
}

func (r *clientRuntime) handleMessage(t protocol.MessageType, payload []byte) {
	c := r.client
	switch t {
	case protocol.MessageTypeUDPTunnel:
		r.handleUDPTunnel(payload)
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
		r.handleTextMessage(tm)
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
		r.handleCryptSetup(cs)
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

func (r *clientRuntime) handleTextMessage(tm *protocol.TextMessage) {
	if protocol.EnableDebug {
		log.Printf("[client] text message from actor=%d: %s", tm.Actor, tm.Message)
	}
	r.client.events.emitTextMessage(tm.Actor, tm.Message)
}

func (r *clientRuntime) handleUDPTunnel(payload []byte) {
	if protocol.EnableDebug {
		log.Printf("[client] UDPTunnel received: len=%d", len(payload))
	}
	if r.client.audio == nil {
		if protocol.EnableDebug {
			log.Printf("[client] audio is nil, dropping packet")
		}
		return
	}
	if err := r.client.audio.ProcessPacket(payload); err != nil {
		if protocol.EnableDebug {
			log.Printf("[client] audio packet error: %v", err)
		}
	}
}

func (r *clientRuntime) handleCryptSetup(cs *protocol.CryptSetup) {
	c := r.client
	if len(cs.Key) >= 16 && len(cs.ServerNonce) >= 16 {
		c.cryptoKey = cs.Key
		c.cryptoServerNonce = cs.ServerNonce
		clientNonce, err := generateNonce()
		if err != nil {
			log.Printf("[client] failed to generate nonce: %v", err)
			return
		}
		c.cryptoClientNonce = clientNonce[:16]
		reply := &protocol.CryptSetup{Key: cs.Key, ClientNonce: clientNonce}
		replyPayload, err := reply.Marshal()
		if err == nil {
			c.conn.WriteFrame(protocol.MessageTypeCryptSetup, replyPayload)
		}
		if c.audio != nil && len(c.cryptoKey) >= 16 {
			c.audio.SetCrypto(c.cryptoKey[:16], c.cryptoClientNonce, c.cryptoServerNonce[:16])
		}
	}
}

func (r *clientRuntime) pingLoop() {
	c := r.client
	defer c.wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.closeCh:
			return
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			ping := &protocol.Ping{Timestamp: uint64(time.Now().Unix())}
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
