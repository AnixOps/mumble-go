package identity

import "crypto/tls"

// Provider supplies client TLS identities.
//
// Implementations may load certificates from local disk today and from
// cloud-backed or remote-signing systems in the future.
type Provider interface {
	TLSCertificate() (*tls.Certificate, error)
}
