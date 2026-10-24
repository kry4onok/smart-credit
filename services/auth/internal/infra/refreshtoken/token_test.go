package refreshtoken

import (
	"crypto/sha256"
	"strings"
	"testing"
)

func TestGenerateAndHash(t *testing.T) {
	tokens := New()
	seen := make(map[string]bool)
	for range 20 {
		secret, err := tokens.Generate()
		if err != nil {
			t.Fatal(err)
		}
		hash, err := tokens.Hash(secret.Token)
		if err != nil || len(secret.Token) != 43 || hash != secret.Hash || hash != sha256.Sum256([]byte(secret.Token)) || seen[secret.Token] {
			t.Fatal("invalid or reused refresh secret")
		}
		seen[secret.Token] = true
	}
	for _, raw := range []string{"", "short", strings.Repeat("A", 42), strings.Repeat("A", 44), strings.Repeat("!", 43), strings.Repeat("A", 42) + "B"} {
		if _, err := tokens.Hash(raw); err == nil {
			t.Fatal("malformed token accepted")
		}
	}
}
