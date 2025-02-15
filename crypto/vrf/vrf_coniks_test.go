//go:build coniks
// +build coniks

package vrf

import (
	"bytes"
	"testing"

	"crypto/ed25519"

	coniks "github.com/coniks-sys/coniks-go/crypto/vrf"
	"github.com/stretchr/testify/require"
)

func TestKeyPairCompatibilityConiks(t *testing.T) {
	secret := [SEEDBYTES]byte{}
	privateKey, _ := coniks.GenerateKey(bytes.NewReader(secret[:]))
	publicKey, _ := privateKey.Public()

	privateKey2 := ed25519.PrivateKey(make([]byte, ed25519.PrivateKeySize))
	copy(privateKey2[:], privateKey[:])
	publicKey2 := privateKey2.Public().(ed25519.PublicKey)
	if !bytes.Equal(publicKey, publicKey2[:]) {
		t.Error("public key is not matched: using same private key which is generated by coniks",
			"coniks.Public", enc(publicKey), "ed25519.Public", enc(publicKey2[:]))
	}

	privateKey2 = ed25519.NewKeyFromSeed(secret[:])
	publicKey2, _ = privateKey2.Public().(ed25519.PublicKey)

	copy(privateKey, privateKey2[:])
	publicKey, _ = privateKey.Public()
	if !bytes.Equal(publicKey, publicKey2[:]) {
		t.Error("public key is not matched: using same private key which is generated by ed25519",
			"coniks.Public", enc(publicKey), "ed25519.Public", enc(publicKey2[:]))
	}

}

func TestProveAndVerify_ConiksByCryptoED25519(t *testing.T) {
	secret := [SEEDBYTES]byte{}
	privateKey := ed25519.NewKeyFromSeed(secret[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	verified, err := proveAndVerify(t, privateKey, publicKey)
	if err != nil {
		t.Fatalf("failed to verify: %s", err)
	}
	//
	// "un-verified" when using crypto ED25519
	// If you want to use coniks, you should use coniks ED25519
	//
	require.False(t, verified)
}

func TestProveAndVerify_ConiksByConiksED25519(t *testing.T) {
	secret := [SEEDBYTES]byte{}
	privateKey, _ := coniks.GenerateKey(bytes.NewReader(secret[:]))
	publicKey, _ := privateKey.Public()

	verified, err := proveAndVerify(t, privateKey, publicKey)
	if err != nil {
		t.Fatalf("failed to verify: %s", err)
	}
	//
	// verified when using coniks ED25519
	//
	require.True(t, verified)
}
