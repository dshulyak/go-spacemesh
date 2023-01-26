package transactions

import (
	"context"
	"time"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/genvm/sdk"
	"github.com/spacemeshos/go-spacemesh/genvm/sdk/wallet"
	"github.com/spacemeshos/go-spacemesh/signing"
)

type singleSig struct {
	state   *StateModel
	est     *Estimator
	private ed25519.PrivateKey
	genesis types.Hash20

	n      int
	period time.Duration
	last   time.Time
}

func (s *singleSig) Next(ctx context.Context) ([][]byte, error) {
	if !s.last.IsZero() {
		since := time.Since(s.last)
		if d := since - s.period; d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		s.last = time.Now()
	}

	if s.state.needsSpawn() {
		txs := [][]byte{
			wallet.SelfSpawn(signing.PrivateKey(s.private), nonce),
		}
		return txs, nil
	}
	if !s.state.waitSpawned(ctx) {
		return nil, ctx.Err()
	}
	var txs [][]byte
	for i := 0; i < s.n; i++ {
		txs = append(txs, wallet.Spend(
			signing.PrivateKey(s.private),
			types.Address{},
			10, nonce,
			sdk.WithGenesisID(s.genesis),
		))
	}
	return txs, nil
}
