package transport

import (
	"testing"
)

func TestConnCloseNil(t *testing.T) {
	// Test that Close doesn't panic on nil connection
	c := &Conn{}
	if err := c.Close(); err != nil {
		t.Errorf("Close on nil conn should return nil, got: %v", err)
	}
}

func TestNilConnWriteFrame(t *testing.T) {
	c := &Conn{}
	err := c.WriteFrame(0, []byte("test"))
	if err == nil {
		t.Error("WriteFrame on nil conn should error")
	}
}

func TestNilConnReadFrame(t *testing.T) {
	c := &Conn{}
	_, _, err := c.ReadFrame()
	if err == nil {
		t.Error("ReadFrame on nil conn should error")
	}
}
