package signing

import (
	"bytes"
	"fmt"

	"github.com/spacemeshos/ed25519"
)

var _ Signer = VRFSigner{}

// VRFSigner is a signer for VRF purposes
type VRFSigner struct {
	privateKey []byte
}

// Sign signs a message for VRF purposes
func (s VRFSigner) Sign(msg []byte) []byte {
	return ed25519.Sign(s.privateKey, msg)
}

// NewVRFSigner creates a new VRFSigner from a 32-byte seed
func NewVRFSigner(seed []byte) (*VRFSigner, []byte, error) {
	if len(seed) < ed25519.SeedSize {
		return nil, nil, fmt.Errorf("seed must be >=%d bytes (len(seed)=%d)", ed25519.SeedSize, len(seed))
	}
	vrfPub, vrfPriv, err := ed25519.GenerateKey(bytes.NewReader(seed))
	if err != nil {
		return nil, nil, err
	}

	return &VRFSigner{privateKey: vrfPriv}, vrfPub, nil
}

// VRFVerify verifies a message and signature, given a public key
func VRFVerify(pub, msg, sig []byte) bool {
	return ed25519.Verify(pub, msg, sig)
}

var _ Verifier = VRFVerifier{}

type VRFVerifier struct{}

func (VRFVerifier) Verify(pub *PublicKey, msg, sig []byte) bool {
	return VRFVerify(ed25519.PublicKey(pub.Bytes()), msg, sig)
}

func (VRFVerifier) Extract(msg, sig []byte) (*PublicKey, error) {
	pub, err := ed25519.ExtractPublicKey(msg, sig)
	if err != nil {
		return nil, err
	}
	return &PublicKey{pub: pub}, nil
}
