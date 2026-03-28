package protocol

// Envelope is a raw control packet.
type Envelope struct {
	Type    MessageType
	Payload []byte
}
