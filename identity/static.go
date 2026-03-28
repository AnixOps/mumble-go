package identity

import "crypto/tls"

// StaticProvider wraps an already-loaded TLS certificate.
type StaticProvider struct {
	Certificate tls.Certificate
}

func (p StaticProvider) TLSCertificate() (*tls.Certificate, error) {
	cert := p.Certificate
	return &cert, nil
}
