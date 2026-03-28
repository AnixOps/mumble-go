package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type Metadata struct {
	Version         int       `json:"version"`
	Profile         string    `json:"profile"`
	Fingerprint     string    `json:"fingerprint"`
	CertificateSHA1 string    `json:"certificate_sha1"`
	Subject         string    `json:"subject"`
	Algorithm       string    `json:"algorithm"`
	CreatedAt       time.Time `json:"created_at"`
}

type LocalStore struct {
	BaseDir string
	Profile string
}

func NewLocalStore(baseDir, profile string) *LocalStore {
	return &LocalStore{BaseDir: baseDir, Profile: profile}
}

func DefaultBaseDir() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(cfgDir, "mumble-go", "identities"), nil
}

func (s *LocalStore) profileDir() (string, error) {
	if s.Profile == "" {
		return "", fmt.Errorf("identity: empty profile")
	}
	base := s.BaseDir
	if base == "" {
		var err error
		base, err = DefaultBaseDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, s.Profile), nil
}

func (s *LocalStore) keyPath() (string, error) {
	dir, err := s.profileDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client.key"), nil
}

func (s *LocalStore) certPath() (string, error) {
	dir, err := s.profileDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client.crt"), nil
}

func (s *LocalStore) metaPath() (string, error) {
	dir, err := s.profileDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "meta.json"), nil
}

func (s *LocalStore) TLSCertificate() (*tls.Certificate, error) {
	return s.LoadOrCreate()
}

func (s *LocalStore) LoadOrCreate() (*tls.Certificate, error) {
	cert, err := s.Load()
	if err == nil {
		return cert, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.Create(); err != nil {
		return nil, err
	}
	return s.Load()
}

func (s *LocalStore) Load() (*tls.Certificate, error) {
	certPath, err := s.certPath()
	if err != nil {
		return nil, err
	}
	keyPath, err := s.keyPath()
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("identity: empty certificate chain")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("identity: parse certificate: %w", err)
	}
	cert.Leaf = leaf
	return &cert, nil
}

func (s *LocalStore) Create() error {
	dir, err := s.profileDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("identity: create dir: %w", err)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("identity: generate key: %w", err)
	}
	pub := priv.Public()

	now := time.Now().UTC()
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("identity: serial: %w", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "mumble-go/" + s.Profile,
			Organization: []string{"mumble-go"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, pub, priv)
	if err != nil {
		return fmt.Errorf("identity: create certificate: %w", err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("identity: marshal private key: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	fingerprint := sha256.Sum256(der)
	sha1sum := sha1.Sum(der)
	meta := Metadata{
		Version:         1,
		Profile:         s.Profile,
		Fingerprint:     hex.EncodeToString(fingerprint[:]),
		CertificateSHA1: hex.EncodeToString(sha1sum[:]),
		Subject:         tpl.Subject.String(),
		Algorithm:       "ecdsa-p256",
		CreatedAt:       now,
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("identity: marshal metadata: %w", err)
	}

	certPath, _ := s.certPath()
	keyPath, _ := s.keyPath()
	metaPath, _ := s.metaPath()
	if err := writeFileAtomic(certPath, certPEM, 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(metaPath, append(metaBytes, '\n'), 0o600); err != nil {
		return err
	}
	return nil
}

func (s *LocalStore) Fingerprint() (string, error) {
	meta, err := s.Metadata()
	if err != nil {
		return "", err
	}
	return meta.Fingerprint, nil
}

func (s *LocalStore) Metadata() (*Metadata, error) {
	metaPath, err := s.metaPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var meta Metadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, fmt.Errorf("identity: parse metadata: %w", err)
	}
	return &meta, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("identity: write %s: %w", path, err)
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return fmt.Errorf("identity: chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("identity: rename %s: %w", path, err)
	}
	return nil
}
