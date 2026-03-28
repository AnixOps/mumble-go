package transport

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"mumble-go/protocol"
)

// Conn manages the TLS control connection.
type Conn struct {
	conn net.Conn
}

func Dial(addr string, cfg *tls.Config) (*Conn, error) {
	c, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	return &Conn{conn: c}, nil
}

func (c *Conn) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Conn) WriteFrame(t protocol.MessageType, payload []byte) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("transport: nil connection")
	}
	_, err := c.conn.Write(protocol.MarshalFrame(t, payload))
	return err
}

// Write implements io.Writer for raw data (used for UDPTunnel audio).
func (c *Conn) Write(data []byte) (int, error) {
	if c == nil || c.conn == nil {
		return 0, fmt.Errorf("transport: nil connection")
	}
	return c.conn.Write(data)
}

func (c *Conn) ReadFrame() (protocol.MessageType, []byte, error) {
	if c == nil || c.conn == nil {
		return 0, nil, fmt.Errorf("transport: nil connection")
	}

	header := make([]byte, protocol.ControlHeaderSize)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return 0, nil, err
	}

	t, n := protocol.UnmarshalHeader(header)
	payload := make([]byte, n)
	if n > 0 {
		if _, err := io.ReadFull(c.conn, payload); err != nil {
			return 0, nil, err
		}
	}

	// Debug: hex dump of frame
	protocol.DebugFrame(t, payload)

	return t, payload, nil
}

func (c *Conn) LocalAddr() net.Addr { return c.conn.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }
