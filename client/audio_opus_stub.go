//go:build !opus
// +build !opus

package client

import "errors"

// SetOpusCodecStub returns an error indicating opus is not available.
func (a *Audio) SetOpusCodec() error {
	return errors.New("opus codec not available (compile with -tags=opus)")
}
