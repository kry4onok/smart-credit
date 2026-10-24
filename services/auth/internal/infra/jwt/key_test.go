package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKeyNeverOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth", "private.pem")
	if err := GenerateKey(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private key must have mode 0600")
	}
	if _, err := loadKey(path); err != nil {
		t.Fatal(err)
	}
	if err := GenerateKey(path); err == nil {
		t.Fatal("existing key overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("existing key changed")
	}
}

func TestLoadPKCS1AndRejectInvalidKeys(t *testing.T) {
	m := newManager(t)
	path := filepath.Join(t.TempDir(), "key.pem")
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(m.key)})
	if err := os.WriteFile(path, pkcs1, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadKey(path); err != nil {
		t.Fatal("PKCS#1 rejected:", err)
	}
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalPKCS8PrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	weak, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"not PEM", []byte("private-secret")},
		{"trailing data", append(append([]byte{}, pkcs1...), []byte("private-secret")...)},
		{"leading data", append([]byte("private-secret\n"), pkcs1...)},
		{"multiple keys", append(append([]byte{}, pkcs1...), pkcs1...)},
		{"too large", []byte(strings.Repeat("private-secret", 65536))},
		{"EC", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecDER})},
		{"weak RSA", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(weak)})},
		{"public key", pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&m.key.PublicKey)})},
		{"invalid DER", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("private-secret")})},
		{"encrypted", pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte("private-secret")})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadKey(path); err == nil || strings.Contains(err.Error(), "private-secret") {
				t.Fatal("key was not safely rejected:", err)
			}
		})
	}
	if _, err := loadKey(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("missing key accepted")
	}
}
