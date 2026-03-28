package mumble

import "mumble-go/client"

// Config aliases the headless client configuration.
type Config = client.Config

// Client aliases the headless Mumble client.
type Client = client.Client

// New creates a new headless Mumble client.
func New(cfg Config) *Client { return client.New(cfg) }
