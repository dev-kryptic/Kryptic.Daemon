package sealedbox

import (
	"bytes"
	"crypto/ecdh"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type interopVector struct {
	RecipientKeyID         string `json:"recipientKeyId"`
	RecipientPrivateKeyHex string `json:"recipientPrivateKeyHex"`
	RecipientPublicKeyHex  string `json:"recipientPublicKeyHex"`
	EphemeralPrivateKeyHex string `json:"ephemeralPrivateKeyHex"`
	EphemeralPublicKeyHex  string `json:"ephemeralPublicKeyHex"`
	PlaintextHex           string `json:"plaintextHex"`
	Sealed                 string `json:"sealed"`
}

func loadVector(t *testing.T) interopVector {
	t.Helper()
	data, err := os.ReadFile("testdata/sealed-box-p256.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var v interopVector
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return v
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	return b
}

// The C#-generated vector must open to the expected plaintext.
func TestInteropVector_Open(t *testing.T) {
	v := loadVector(t)

	box, err := Parse(v.Sealed)
	if err != nil {
		t.Fatalf("parse sealed box: %v", err)
	}
	recipient := KeyPair{
		Public:  mustHex(t, v.RecipientPublicKeyHex),
		Private: mustHex(t, v.RecipientPrivateKeyHex),
	}

	plaintext, err := Open(recipient, box)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(plaintext, mustHex(t, v.PlaintextHex)) {
		t.Fatalf("plaintext mismatch: got %x", plaintext)
	}
}

// Sealing with the vector's fixed ephemeral key must reproduce the C# output
// byte for byte - this locks the KDF, nonce derivation and wire format.
func TestInteropVector_SealReproduces(t *testing.T) {
	v := loadVector(t)

	ephemeral, err := ecdh.P256().NewPrivateKey(mustHex(t, v.EphemeralPrivateKeyHex))
	if err != nil {
		t.Fatalf("import ephemeral key: %v", err)
	}

	box, err := sealWithEphemeral(ephemeral, mustHex(t, v.RecipientPublicKeyHex), v.RecipientKeyID, mustHex(t, v.PlaintextHex))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if got := box.Serialize(); got != v.Sealed {
		t.Fatalf("serialized sealed box mismatch:\n got %s\nwant %s", got, v.Sealed)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	recipient, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("the org data key would go here 1")

	box, err := Seal(recipient.Public, "ukey_roundtrip01", plaintext)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	parsed, err := Parse(box.Serialize())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	opened, err := Open(recipient, parsed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("round trip mismatch")
	}
}

func TestOpenWrongRecipientFails(t *testing.T) {
	recipient, _ := GenerateKeyPair()
	attacker, _ := GenerateKeyPair()

	box, err := Seal(recipient.Public, "ukey_wrongrecip1", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(attacker, box); err == nil {
		t.Fatal("expected authentication failure for wrong recipient")
	}
}

func TestOpenTamperedCiphertextFails(t *testing.T) {
	recipient, _ := GenerateKeyPair()
	box, err := Seal(recipient.Public, "ukey_tampered001", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	box.CiphertextWithTag[0] ^= 0x01
	if _, err := Open(recipient, box); err == nil {
		t.Fatal("expected authentication failure for tampered ciphertext")
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	for _, value := range []string{
		"",
		"not-a-sealed-box",
		"sbx.v2.key.AA.BB.CC",
		"env.v1.key.AA.BB.CC",
		"sbx.v1.bad key.AA.BB.CC",
	} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("expected parse failure for %q", value)
		}
	}
}
