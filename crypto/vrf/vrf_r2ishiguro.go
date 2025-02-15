//go:build !libsodium && !coniks
// +build !libsodium,!coniks

package vrf

import (
	"crypto/ed25519"

	r2ishiguro "github.com/r2ishiguro/vrf/go/vrf_ed25519"
)

type vrfEd25519r2ishiguro struct {
}

func init() {
	// if you use build option for other implementation, defaultVrf is overridden by other implementation.
	// It may vrfEd25519libsodium
	if defaultVrf == nil {
		defaultVrf = newVrfEd25519r2ishiguro()
	}
}

func newVrfEd25519r2ishiguro() vrfEd25519r2ishiguro {
	return vrfEd25519r2ishiguro{}
}

func (base vrfEd25519r2ishiguro) Prove(privateKey []byte, message []byte) (Proof, error) {
	publicKey := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	return r2ishiguro.ECVRF_prove(publicKey, privateKey, message)
}

func (base vrfEd25519r2ishiguro) Verify(publicKey []byte, proof Proof, message []byte) (bool, error) {
	return r2ishiguro.ECVRF_verify(publicKey, proof, message)
}

func (base vrfEd25519r2ishiguro) ProofToHash(proof Proof) (Output, error) {
	return r2ishiguro.ECVRF_proof2hash(proof), nil
}
