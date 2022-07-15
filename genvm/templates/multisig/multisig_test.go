package multisig

import (
	"bytes"
	"testing"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-scale"
	"github.com/spacemeshos/go-spacemesh/genvm/core"
)

func TestVerify(t *testing.T) {
	tpl := &MultiSig{
		N: 2,
		M: 3,
	}
	tpl.State.PublicKeys = make([]core.PublicKey, tpl.M)
	pks := make([]ed25519.PrivateKey, tpl.M)
	for i := range tpl.State.PublicKeys {
		pub, pk, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		copy(tpl.State.PublicKeys[i][:], pub)
		pks[i] = pk
	}
	msg := []byte{1, 2, 3}
	hash := core.Hash(msg)
	sigs := make([]Signature, tpl.N)
	copy(sigs[0].Signature[:], ed25519.Sign(pks[0], hash[:]))
	copy(sigs[1].Signature[:], ed25519.Sign(pks[0], hash[:]))
	buf := bytes.NewBuffer(nil)
	enc := scale.NewEncoder(buf)
	_, err := scale.EncodeStructArray(enc, sigs)
	require.NoError(t, err)
	require.False(t, tpl.Verify(&core.Context{}, msg, scale.NewDecoder(buf)))
}
