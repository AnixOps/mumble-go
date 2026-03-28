// Package audio handles Mumble voice encoding, decoding, and crypto.
package audio

// Audio constants for Mumble protocol.
const (
	SampleRate       = 48000 // 48kHz
	Channels         = 1     // Mono
	FrameSize        = 960   // 20ms at 48kHz mono
	FrameDurationMs  = 20    // 20ms frame duration
	SequenceDuration = 10    // 10ms sequence unit

	// Legacy Mumble UDP voice types: 0=CELT-alpha, 1=Ping, 2=Speex, 3=CELT-beta, 4=Opus
	AudioTypeOpus     = 4
	AudioTypeCELTBeta = 3
	AudioTypeSpeex    = 2
)

// VoiceTarget types.
const (
	TargetNormal   = 0 // Normal speech
	TargetWhisper  = 1 // Whisper to channel
	TargetWhisperU = 2 // Whisper to user
)
