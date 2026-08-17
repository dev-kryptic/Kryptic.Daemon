package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// seal builds a well-formed envelope the way the browser does; the daemon
// itself never seals, so this lives in the test.
func seal(t *testing.T, key []byte, keyID string, plaintext, associatedData []byte) string {
	t.Helper()

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, associatedData)

	enc := base64.RawURLEncoding
	return strings.Join([]string{
		formatVersion, keyID, enc.EncodeToString(nonce), enc.EncodeToString(ciphertext),
	}, ".")
}

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestOpenRoundTrip(t *testing.T) {
	key := testKey(t)
	associatedData := SecretContext("A1B2C3D4-0000-0000-0000-000000000001", "a1b2c3d4-0000-0000-0000-000000000002")
	serialized := seal(t, key, "org-key-1", []byte("postgres://user:pw@host/db"), associatedData)

	plaintext, err := Open(key, serialized, associatedData)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(plaintext) != "postgres://user:pw@host/db" {
		t.Fatalf("plaintext = %q", plaintext)
	}
}

func TestOpenRejectsWrongAssociatedData(t *testing.T) {
	key := testKey(t)
	right := SecretContext("def-1", "env-1")
	serialized := seal(t, key, "org-key-1", []byte("value"), right)

	if _, err := Open(key, serialized, SecretContext("def-1", "env-2")); err == nil {
		t.Fatal("expected authentication failure when the envelope is bound to another environment")
	}
}

func TestOpenRejectsWrongKey(t *testing.T) {
	right := testKey(t)
	serialized := seal(t, right, "org-key-1", []byte("value"), nil)

	if _, err := Open(testKey(t), serialized, nil); err == nil {
		t.Fatal("expected authentication failure with the wrong key")
	}
}

func TestParseRejectsMalformedEnvelopes(t *testing.T) {
	enc := base64.RawURLEncoding
	shortNonce := "v1.k." + enc.EncodeToString(make([]byte, 4)) + "." + enc.EncodeToString(make([]byte, 32))
	shortCiphertext := "v1.k." + enc.EncodeToString(make([]byte, nonceSize)) + "." + enc.EncodeToString(make([]byte, 8))

	for _, value := range []string{
		"", "v1", "v2.k.a.b", "v1.k.!!!.b", "not an envelope",
		shortNonce, shortCiphertext,
	} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("Parse(%q) accepted a malformed envelope", value)
		}
	}
}

// SecretContext must match the browser (crypto/secrets.ts) and the C# engine
// byte for byte or decryption fails: lowercase ids, exact label layout.
func TestSecretContextFormat(t *testing.T) {
	got := string(SecretContext("ABC-123", "DEF-456"))
	want := "secret:abc-123:env:def-456"
	if got != want {
		t.Fatalf("SecretContext = %q, want %q", got, want)
	}
}
