package transactions

import (
	"context"
	"time"
)

type SimpleWallet struct {
	state *StateModel
	est   *Estimator

	n      int
	period time.Duration
	last   time.Time
}

func (s *SimpleWallet) Next(ctx context.Context) ([][]byte, error) {
	if !s.last.IsZero() {
		if delta := time.Since(s.last) - s.period; delta > 0 {
			select {
			case <-time.After(delta):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	s.last = time.Now()

	var txs [][]byte
	switch s.state.WaitSpawned(ctx) {
	case NeedsSpawn:
		return txs, nil
	case Unknown:
		return nil, ctx.Err()
	case Spawned:
		for i := 0; i < s.n; i++ {
		}
	}
	panic("unreachable")
}
