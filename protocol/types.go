package protocol

// MessageType identifies a Mumble TCP or UDP message.
type MessageType uint16

const (
	MessageTypeVersion     MessageType = 0
	MessageTypeUDPTunnel   MessageType = 1
	MessageTypeAuthenticate MessageType = 2
	MessageTypePing        MessageType = 3
	MessageTypeReject      MessageType = 4
	MessageTypeServerSync  MessageType = 5
	MessageTypeChannelRemove MessageType = 6
	MessageTypeChannelState MessageType = 7
	MessageTypeUserRemove  MessageType = 8
	MessageTypeUserState   MessageType = 9
	MessageTypeBanList     MessageType = 10
	MessageTypeTextMessage MessageType = 11
	MessageTypePermissionDenied MessageType = 12
	MessageTypeACL        MessageType = 13
	MessageTypeQueryUsers  MessageType = 14
	MessageTypeCryptSetup  MessageType = 15
	MessageTypeVoiceTarget MessageType = 19
	MessageTypeCodecVersion MessageType = 21
	MessageTypeRequestBlob MessageType = 23
	MessageTypeServerConfig MessageType = 24
)
