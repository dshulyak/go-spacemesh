package signing

import (
	mrand "math/rand"
	"testing"

	"github.com/spacemeshos/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oasised "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/spacemeshos/go-spacemesh/rand"
)

func TestNewEdSignerFromBuffer(t *testing.T) {
	b := []byte{1, 2, 3}
	_, err := NewEdSignerFromBuffer(b)
	assert.NotNil(t, err)
	assert.Equal(t, "buffer too small", err.Error())
	b = make([]byte, 64)
	_, err = NewEdSignerFromBuffer(b)
	assert.NotNil(t, err)
	assert.Equal(t, "private and public does not match", err.Error())
}

func TestEdSigner_Sign(t *testing.T) {
	ed := NewEdSigner()
	m := make([]byte, 4)
	rand.Read(m)
	sig := ed.Sign(m)
	assert.True(t, ed25519.Verify2(ed25519.PublicKey(ed.PublicKey().Bytes()), m, sig))
}

func TestNewEdSigner(t *testing.T) {
	ed := NewEdSigner()
	assert.Equal(t, []byte(ed.pubKey), []byte(ed.privKey[32:]))
}

func TestEdSigner_ToBuffer(t *testing.T) {
	ed := NewEdSigner()
	buff := ed.ToBuffer()
	ed2, err := NewEdSignerFromBuffer(buff)
	assert.Nil(t, err)
	assert.Equal(t, ed.privKey, ed2.privKey)
	assert.Equal(t, ed.pubKey, ed2.pubKey)
}

func TestPublicKey_ShortString(t *testing.T) {
	pub := NewPublicKey([]byte{1, 2, 3})
	assert.Equal(t, "010203", pub.String())
	assert.Equal(t, "01020", pub.ShortString())

	pub = NewPublicKey([]byte{1, 2})
	assert.Equal(t, pub.String(), pub.ShortString())
}

func BenchmarkVerify(b *testing.B) {
	rng := mrand.New(mrand.NewSource(1001))
	pubk, pk, err := ed25519.GenerateKey(rng)
	require.NoError(b, err)
	msg := make([]byte, 32)
	rng.Read(msg)

	sig := ed25519.Sign(pk, msg)
	sigplusplus := ed25519.Sign2(pk, msg)

	b.Run("Recover", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ed25519.ExtractPublicKey(msg, sigplusplus)
			if err != nil {
				b.FailNow()
			}
		}
	})
	b.Run("Verify", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !ed25519.Verify(pubk, msg, sig) {
				b.FailNow()
			}
		}
	})
	b.Run("VerifyOasis", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !oasised.Verify(oasised.PublicKey(pubk), msg, sig) {
				b.FailNow()
			}
		}
	})
}
