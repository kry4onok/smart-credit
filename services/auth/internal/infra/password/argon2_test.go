package password

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h := New()
	const password = "  длинный пароль 🔐  "
	first, err := h.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	// Same PHC shape as the users.password_hash constraint in the migration.
	pattern := regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=3,p=4\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	if first == second || !pattern.MatchString(first) || !pattern.MatchString(second) {
		t.Fatal("hashes must use independent salts and the supported PHC format")
	}
	for _, tc := range []struct {
		password string
		want     bool
	}{{password, true}, {strings.TrimSpace(password), false}, {"wrong", false}, {"", false}} {
		ok, err := h.Verify(first, tc.password)
		if err != nil || ok != tc.want {
			t.Fatalf("verify result=%v, error=%v, want=%v", ok, err, tc.want)
		}
	}
	if _, err := h.Hash(""); err == nil {
		t.Fatal("empty password accepted")
	}
}

func TestVerifyRejectsMalformedAndUnboundedHashes(t *testing.T) {
	const valid = "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	for _, hash := range []string{
		"", strings.Repeat("x", 257), strings.Replace(valid, "argon2id", "argon2i", 1),
		strings.Replace(valid, "v=19", "v=16", 1),
		strings.Replace(valid, "m=65536", "m=4294967295", 1),
		strings.Replace(valid, "m=65536", "m=4294967296", 1),
		strings.Replace(valid, "m=65536", "m=1", 1),
		strings.Replace(valid, "m=65536", "m=+65536", 1),
		strings.Replace(valid, "t=3", "t=0", 1), strings.Replace(valid, "t=3", "t=4", 1),
		strings.Replace(valid, "p=4", "p=0", 1), strings.Replace(valid, "p=4", "p=256", 1),
		strings.Replace(valid, "m=65536,t=3,p=4", "t=3,m=65536,p=4", 1),
		strings.Replace(valid, "m=65536,t=3,p=4", "m=65536,t=3,p=4,x=1", 1),
		strings.Replace(valid, "$AAAAAAAAAAAAAAAAAAAAAA$", "$short$", 1),
		valid + "=", valid + "$", valid[:len(valid)-1], valid[:len(valid)-1] + "B",
	} {
		if ok, err := New().Verify(hash, "private-password"); ok || !errors.Is(err, ErrInvalidHash) || strings.Contains(err.Error(), "private-password") {
			t.Errorf("malformed hash accepted: match=%v, error=%v", ok, err)
		}
	}
}
