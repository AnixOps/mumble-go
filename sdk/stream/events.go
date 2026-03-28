package stream

import "time"

// StreamEvents contains callbacks for stream lifecycle events.
type StreamEvents struct {
	// OnConnect is called when the stream becomes active (first start).
	OnConnect func()

	// OnDisconnect is called when the stream stops (Stop or error).
	OnDisconnect func(err error)

	// OnReconnecting is called when a reconnection is attempted.
	// attempt is the current attempt number (starting at 1).
	// nextDelay is the duration until the next attempt.
	OnReconnecting func(attempt int, nextDelay time.Duration)

	// OnError is called for non-fatal errors (e.g., source errors).
	OnError func(err error)

	// OnVADChange is called when voice activity detection state changes.
	// speaking=true when audio energy exceeds threshold.
	OnVADChange func(speaking bool)

	// OnSourceError is called when a specific audio source encounters an error.
	OnSourceError func(id string, err error)

	// OnMetadataSet is called when stream metadata is successfully sent.
	OnMetadataSet func(meta *StreamMetadata)
}
