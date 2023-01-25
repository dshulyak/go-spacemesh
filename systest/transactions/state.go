package transactions

import (
	"context"
	"sync"
)

const (
	methodSpawn = 0
	methodSpend = 16
	methodDrain = 17
)

type state struct {
	mu                  sync.Mutex
	coinbase            string
	applied, optimistic struct {
		balance, nextNonce uint64
	}

	spawned chan struct{}
}

type reward struct {
	layer    uint32
	coinbase string
	amount   uint64
}

type tx struct {
	layer     uint32
	principal string
	updated   []string
	template  string
	method    uint8
	failed    bool
	fee       uint64
	nonce     uint64
	raw       []byte
}

func (s *state) OnReward(reward *reward) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if reward.coinbase != s.coinbase {
		return
	}
	s.applied.balance += reward.amount
}

func (s *state) OnTx(tx *tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	relevant := false
	for _, addr := range tx.updated {
		relevant := addr == s.coinbase
		if relevant {
			break
		}
	}
	if !relevant {
		return
	}
	if s.coinbase == tx.principal {
		s.applied.balance -= tx.fee
		s.applied.nextNonce = tx.nonce + 1
	}
	if tx.failed {
		return
	}
	s.dispatchTx(tx)
}

func (s *state) dispatchTx(tx *tx) {
	switch tx.method {
	case methodSpawn:
		if s.coinbase == tx.principal && len(tx.updated) == 1 {
			close(s.spawned)
		} else if s.coinbase != tx.principal && len(tx.updated) == 2 {
			close(s.spawned)
		}
	case methodSpend:
	case methodDrain:
	}
}

func (s *state) onTransfer(from, to string, amount uint64) {
	switch s.coinbase {
	case from:
		s.applied.balance -= amount
	case to:
		s.applied.balance += amount
	}
}

func (s *state) WaitSpawned(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.spawned:
		return true
	}
}

func (s *state) NeedsSpawn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isSpawned() {
		return true
	}
	// check if spawn is in the tx list, and in this case simply wait
	return false
}

func (s *state) isSpawned() bool {
	select {
	case <-s.spawned:
		return true
	default:
		return false
	}
}
