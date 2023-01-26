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

type Reward struct {
	Layer    uint32
	Coinbase string
	Amount   uint64
}

type Tx struct {
	Layer     uint32
	Principal string
	Updated   []string
	Template  string
	Method    uint8
	Failed    bool
	Fee       uint64
	Nonce     uint64
	Raw       []byte
}

type StateModel struct {
	mu                  sync.Mutex
	coinbase            string
	applied, optimistic struct {
		balance, nextNonce uint64
	}
	from, to, max uint32
	layers        map[uint32]layer

	spawned     chan struct{}
	waitBalance struct {
		amount uint64
		waiter chan struct{}
	}
}

type layer struct {
	txs    map[string]*Tx
	reward *Reward
}

func (s *StateModel) Address() string {
	return s.coinbase
}

func (s *StateModel) OnReward(reward *Reward) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if reward.Coinbase != s.coinbase {
		return
	}
	s.applied.balance += reward.Amount
}

func (s *StateModel) OnTx(tx *Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	relevant := false
	for _, addr := range tx.Updated {
		relevant := addr == s.coinbase
		if relevant {
			break
		}
	}
	if !relevant {
		return
	}
	if s.coinbase == tx.Principal {
		s.applied.balance -= tx.Fee
		if tx.Nonce >= s.applied.nextNonce {
			s.applied.nextNonce = tx.Nonce + 1
		}
	}
	if tx.Failed {
		return
	}
	switch tx.Method {
	case methodSpawn:
		if s.coinbase == tx.Principal && len(tx.Updated) == 1 {
			close(s.spawned)
		} else if s.coinbase != tx.Principal && len(tx.Updated) == 2 {
			close(s.spawned)
		}
	case methodSpend:
	case methodDrain:
	}
}

func (s *StateModel) PullForTx(ctx context.Context, amount uint64) *uint64 {
	s.mu.Lock()

	s.mu.Unlock()

}

func (s *StateModel) onTransfer(from, to string, amount uint64) {
	switch s.coinbase {
	case from:
		s.applied.balance -= amount
	case to:
		s.applied.balance += amount
		if s.waitBalance.waiter != nil && s.waitBalance.amount >= s.optimistic.balance {
			s.waitBalance.waiter = nil
			s.waitBalance.amount = 0
			close(s.waitBalance.waiter)
		}
	}
}

func (s *StateModel) waitSpawned(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.spawned:
		return true
	}
}

func (s *StateModel) needsSpawn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isSpawned() {
		return true
	}
	// check if spawn is in the tx list, and in this case simply wait
	return false
}

func (s *StateModel) isSpawned() bool {
	select {
	case <-s.spawned:
		return true
	default:
		return false
	}
}

func (s *StateModel) waitForAmount(ctx context.Context, amount uint64) bool {
	s.mu.Lock()
	if s.waitBalance.waiter == nil {
		s.waitBalance.waiter = make(chan struct{})
		s.waitBalance.amount = amount
	} else {
		s.waitBalance.amount += amount
	}
	waiter := s.waitBalance.waiter
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-waiter:
		return true
	}
}
