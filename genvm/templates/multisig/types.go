package multisig

import "github.com/spacemeshos/go-spacemesh/genvm/core"

//go:generate scalegen

// SpawnArguments ...
type SpawnArguments struct {
	PublicKeys []core.PublicKey
}

// SpendArguments ...
type SpendArguments struct {
	Destination core.Address
	Amount      uint64
}

// SpendPayload ...
type SpendPayload struct {
	Arguments SpendArguments
	Nonce     core.Nonce
	GasPrice  uint64
}

// SpawnPayload ...
type SpawnPayload struct {
	Arguments SpawnArguments
	GasPrice  uint64
}

// Signature ...
type Signature struct {
	PublicKeyRef uint8
	Signature    [64]byte
}
