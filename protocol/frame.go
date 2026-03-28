package protocol

import "encoding/binary"

const ControlHeaderSize = 6

// Frame is a single framed Mumble control packet.
type Frame struct {
	Type    MessageType
	Payload []byte
}

func MarshalFrame(t MessageType, payload []byte) []byte {
	buf := make([]byte, ControlHeaderSize+len(payload))
	binary.BigEndian.PutUint16(buf[0:2], uint16(t))
	binary.BigEndian.PutUint32(buf[2:6], uint32(len(payload)))
	copy(buf[6:], payload)
	return buf
}

func UnmarshalHeader(header []byte) (MessageType, uint32) {
	return MessageType(binary.BigEndian.Uint16(header[0:2])), binary.BigEndian.Uint32(header[2:6])
}
