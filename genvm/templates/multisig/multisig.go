package multisig

import (
	"fmt"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"

	"github.com/spacemeshos/go-scale"
	"github.com/spacemeshos/go-spacemesh/genvm/core"
)

const (
	methodSpawn = 0
	methodSpend = 1
)

type MultiSig struct {
	N, M  int
	State state
}

func (s MultiSig) Spawn(decoder *scale.Decoder) (*MultiSig, error) {
	s.State.PublicKeys = make([]core.PublicKey, s.M)
	_, err := scale.DecodeStructArray(decoder, s.State.PublicKeys)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", core.ErrMalformed, err)
	}
	return &s, nil
}

type state struct {
	PublicKeys []core.PublicKey
}

// MaxSpend returns amount specified in the SpendArguments for Spend method.
func (s *MultiSig) MaxSpend(method uint8, args any) (uint64, error) {
	switch method {
	case methodSpawn:
		return 0, nil
	case methodSpend:
		return args.(*SpendArguments).Amount, nil
	default:
		return 0, fmt.Errorf("%w: unknown method %d", core.ErrMalformed, method)
	}
}

// Verify that transaction is signed by the owner of the PublicKey using ed25519.
func (s *MultiSig) Verify(ctx *core.Context, payload []byte, decoder *scale.Decoder) bool {
	sigs := make([]Signature, s.N)
	_, err := scale.DecodeStructArray(decoder, sigs)
	if err != nil {
		return false
	}
	hash := core.Hash(payload)
	batch := ed25519.NewBatchVerifier()
	for i := range sigs {
		sig := &sigs[i]
		if sig.PublicKeyRef > uint8(len(s.State.PublicKeys)-1) {
			return false
		}
		batch.Add(s.State.PublicKeys[sig.PublicKeyRef][:], hash[:], sig.Signature[:])
	}
	all, _ := batch.Verify(nil)
	return all
}

// Spend transfers an amount to the address specified in SpendArguments.
func (s *MultiSig) Spend(ctx *core.Context, args *SpendArguments) error {
	return ctx.Transfer(args.Destination, args.Amount)
}
