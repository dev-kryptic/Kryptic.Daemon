package possession

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/dev-kryptic/Kryptic.Encryption.Go/sealedbox"
)

func TestSignChallengeVerifies(t *testing.T) {
	pair, err := sealedbox.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding
	challenge := make([]byte, 32)
	for i := range challenge {
		challenge[i] = byte(i + 1)
	}
	challengeB64 := enc.EncodeToString(challenge)
	privB64 := enc.EncodeToString(pair.Private)

	sigB64, err := SignChallenge(privB64, challengeB64)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := enc.DecodeString(sigB64)
	if err != nil || len(sig) != 64 {
		t.Fatalf("signature length %d", len(sig))
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), pair.Public)
	if x == nil {
		t.Fatal("bad public key")
	}
	pub := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	sum := sha256.Sum256(challenge)
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&pub, sum[:], r, s) {
		t.Fatal("signature did not verify")
	}
}
