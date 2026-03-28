// connect is a minimal Mumble client that connects to a server and logs the handshake.
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"mumble-go/client"
	"mumble-go/identity"
	"mumble-go/protocol"
)

func main() {
	addr := "mumble.hotxiang.cn:64738"
	if a := os.Getenv("MUMBLE_ADDR"); a != "" {
		addr = a
	}

	username := "mumble-go-test"
	if u := os.Getenv("MUMBLE_USER"); u != "" {
		username = u
	}
	password := os.Getenv("MUMBLE_PASS")
	identityProfile := os.Getenv("MUMBLE_IDENTITY_PROFILE")
	identityDir := os.Getenv("MUMBLE_IDENTITY_DIR")

	// Disable protocol debug logging for clean output
	protocol.EnableDebug = false

	cfg := client.Config{
		Address:  addr,
		Username: username,
		Password: password,
		TLS: &tls.Config{
			ServerName:         "mumble.hotxiang.cn",
			InsecureSkipVerify: true,
		},
	}
	if identityProfile != "" {
		store := identity.NewLocalStore(identityDir, identityProfile)
		cfg.Identity = store
		if meta, err := store.Metadata(); err == nil {
			fmt.Printf("Using identity profile %s\n  sha256=%s\n  sha1=%s\n  subject=%s\n", identityProfile, meta.Fingerprint, meta.CertificateSHA1, meta.Subject)
		}
	}

	c := client.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Printf("Connecting to %s as %s...\n", addr, username)
	if err := c.Connect(ctx); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("OK! session=%d opus=%v\n", c.State().SelfSession(), c.SupportsOpus())
	if self, ok := c.State().SnapshotUsers()[c.State().SelfSession()]; ok {
		fmt.Printf("Identity state: user_id=%d cert_hash=%s\n", self.UserID, self.CertificateHash)
	}

	channels := c.State().SnapshotChannels()
	fmt.Printf("\nChannels (%d):\n", len(channels))
	for id, ch := range channels {
		fmt.Printf("  [%2d] %s", id, ch.Name)
		if ch.ParentID != 0 {
			fmt.Printf(" (parent=%d)", ch.ParentID)
		}
		fmt.Println()
	}

	users := c.State().SnapshotUsers()
	fmt.Printf("\nUsers (%d):\n", len(users))
	for id, u := range users {
		fmt.Printf("  [%2d] %s (channel=%d", id, u.Name, u.ChannelID)
		if u.Muted {
			fmt.Print(" muted")
		}
		if u.Deafened {
			fmt.Print(" deaf")
		}
		if u.SelfMute {
			fmt.Print(" self-muted")
		}
		if u.SelfDeaf {
			fmt.Print(" self-deaf")
		}
		fmt.Println(")")
	}

	fmt.Println("\nConnected. Waiting (press Ctrl+C to disconnect)...")
	select {}
}
