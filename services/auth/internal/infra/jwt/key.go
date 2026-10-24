package jwt

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GenerateKey creates a persistent PKCS#8 RSA key. Existing keys are never replaced.
func GenerateKey(path string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate RSA key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("encode RSA key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create key directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create key file (existing keys cannot be overwritten): %w", err)
	}
	writeErr := pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der})
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("write key file: %w", err)
	}
	return nil
}

func loadKey(path string) (*rsa.PrivateKey, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open JWT private key: %w", err)
	}
	defer func() { _ = f.Close() }()
	const maxKeyBytes = 64 << 10
	data, err := io.ReadAll(io.LimitReader(f, maxKeyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JWT private key: %w", err)
	}
	if len(data) > maxKeyBytes {
		return nil, errors.New("JWT private key file is too large")
	}
	data = bytes.TrimSpace(data)
	block, rest := pem.Decode(data)
	if block == nil || !bytes.HasPrefix(data, []byte("-----BEGIN "+block.Type+"-----")) || len(bytes.TrimSpace(rest)) != 0 || len(block.Headers) != 0 {
		return nil, errors.New("JWT private key must contain one unencrypted PEM block")
	}
	var key *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		var parsed any
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err == nil {
			var ok bool
			key, ok = parsed.(*rsa.PrivateKey)
			if !ok {
				return nil, errors.New("JWT private key must be RSA")
			}
		}
	default:
		return nil, errors.New("JWT private key must be PKCS#1 or PKCS#8 RSA")
	}
	if err != nil {
		return nil, errors.New("cannot parse JWT private key")
	}
	if key.N.BitLen() < 2048 || key.N.BitLen() > 8192 || key.E < 3 || key.E > 2147483647 || key.E%2 == 0 {
		return nil, errors.New("JWT RSA key must have 2048 to 8192 bits and a valid exponent")
	}
	if err := key.Validate(); err != nil {
		return nil, errors.New("invalid JWT RSA private key")
	}
	key.Precompute()
	return key, nil
}
