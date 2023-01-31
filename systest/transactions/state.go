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
	mu       sync.Mutex
	coinbase string
	applied  struct {
		layer              uint32
		balance, nextNonce uint64
	}
	optimistic struct {
		balance, nextNonce uint64
	}
	from, to, max uint32
	layers        map[uint32]layer

	spawned     chan struct{}
	waitBalance struct {
		amount uint64
		c      chan struct{}
	}
	reorg chan struct{}
}

type layer struct {
	txs    map[string]*Tx
	reward *Reward
}

func (s *StateModel) Coinbase() string {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func isRelevant(tx *Tx, coinbase string) bool {
	for _, addr := range tx.Updated {
		if addr == coinbase {
			return true
		}
	}
	return false
}

func (s *StateModel) OnTx(tx *Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isRelevant(tx, s.coinbase) {
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

func (s *StateModel) OnLayer(layer uint32, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// reorg is started when layer with a different hash is received
	// reorg is finished wnen all previously applied layers got an event

	// it is very hard to understand what should be a behavior for the wallet
	reorg := false
	if reorg {
		s.reorg = make(chan struct{})
	}
}

type Expected struct {
	Applied struct {
		Layer              uint32
		Balance, NextNonce uint64
	}
	Optimistic struct {
		Balance, NextNonce uint64
	}
}

func (s *StateModel) Expect() Expected {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected := Expected{}
	expected.Applied.Layer = s.applied.layer
	expected.Applied.Balance = s.applied.balance
	expected.Applied.NextNonce = s.applied.nextNonce
	expected.Optimistic.Balance = s.applied.balance
	expected.Optimistic.NextNonce = s.applied.nextNonce
	return expected
}

func (s *StateModel) onTransfer(from, to string, amount uint64) {
	switch s.coinbase {
	case from:
		s.applied.balance -= amount
	case to:
		s.applied.balance += amount
		if s.waitBalance.c != nil && s.waitBalance.amount >= s.optimistic.balance {
			s.waitBalance.c = nil
			s.waitBalance.amount = 0
			close(s.waitBalance.c)
		}
	}
}

type SpawnState int

const (
	Unknown SpawnState = iota
	NeedsSpawn
	Spawned
)

func (s *StateModel) WaitSpawned(ctx context.Context) SpawnState {
	select {
	case <-ctx.Done():
		return Unknown
	case <-s.spawned:
		return Spawned
	}
}

func (s *StateModel) waitForAmount(ctx context.Context, amount uint64) bool {
	s.mu.Lock()
	if s.waitBalance.c == nil {
		s.waitBalance.c = make(chan struct{})
		s.waitBalance.amount = amount
	} else {
		s.waitBalance.amount += amount
	}
	waiter := s.waitBalance.c
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-waiter:
		return true
	}
}
