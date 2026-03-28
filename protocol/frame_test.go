package protocol

import (
	"testing"
)

func TestControlHeaderSize(t *testing.T) {
	if ControlHeaderSize != 6 {
		t.Errorf("ControlHeaderSize = %d, want 6", ControlHeaderSize)
	}
}

func TestMarshalFrame(t *testing.T) {
	tests := []struct {
		msgType MessageType
		payload []byte
		wantLen int
	}{
		{MessageTypeVersion, []byte{0x08, 0x01}, 8},
		{5, []byte{0x00}, 7},
		{15, []byte("testpayload"), 6 + 11},
	}
	for _, tt := range tests {
		data := MarshalFrame(tt.msgType, tt.payload)
		if len(data) != tt.wantLen {
			t.Errorf("MarshalFrame(type=%d, len=%d) len=%d, want %d", tt.msgType, len(tt.payload), len(data), tt.wantLen)
		}
		// Check header fields round-trip
		typ, payloadLen := UnmarshalHeader(data)
		if tt.msgType != typ || uint32(len(tt.payload)) != payloadLen {
			t.Errorf("MarshalFrame round-trip: got type=%d len=%d, want type=%d len=%d", typ, payloadLen, tt.msgType, len(tt.payload))
		}
	}
}

func TestUnmarshalHeader(t *testing.T) {
	// MessageType=0 (Version), payload len=4
	h0 := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x04}
	typ, payloadLen := UnmarshalHeader(h0)
	if typ != 0 || payloadLen != 4 {
		t.Errorf("UnmarshalHeader(%x) = type=%d len=%d, want type=0 len=4", h0, typ, payloadLen)
	}

	// MessageType=5, payload len=16
	h1 := []byte{0x00, 0x05, 0x00, 0x00, 0x00, 0x10}
	typ, payloadLen = UnmarshalHeader(h1)
	if typ != 5 || payloadLen != 16 {
		t.Errorf("UnmarshalHeader(%x) = type=%d len=%d, want type=5 len=16", h1, typ, payloadLen)
	}

	// MessageType=15, payload len=0
	h2 := []byte{0x00, 0x0f, 0x00, 0x00, 0x00, 0x00}
	typ, payloadLen = UnmarshalHeader(h2)
	if typ != 15 || payloadLen != 0 {
		t.Errorf("UnmarshalHeader(%x) = type=%d len=%d, want type=15 len=0", h2, typ, payloadLen)
	}
}

func TestMarshalFrameHeaderOnly(t *testing.T) {
	// Test framing without payload
	frame := MarshalFrame(MessageTypePing, nil)
	if len(frame) != 6 {
		t.Errorf("empty frame len=%d, want 6", len(frame))
	}
	typ, payloadLen := UnmarshalHeader(frame)
	if typ != MessageTypePing || payloadLen != 0 {
		t.Errorf("empty frame round-trip: got type=%d len=%d, want type=%d len=0", typ, payloadLen, MessageTypePing)
	}
}
