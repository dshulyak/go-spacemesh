package transactions

import (
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/genvm/core"
	"github.com/spacemeshos/go-spacemesh/genvm/templates/multisig"
	"github.com/spacemeshos/go-spacemesh/genvm/templates/wallet"
)

type Estimator struct {
	StorageFactor uint64
}

type Input struct {
	Template types.Address
	Method   uint8
	Raw      []byte
}

func (e *Estimator) Estimate(input *Input) uint64 {
	var fixed, base uint64
	switch input.Template {
	case wallet.TemplateAddress:
		base = wallet.BaseGas
		switch input.Method {
		case core.MethodSpawn:
			fixed = wallet.FixedGasSpawn
		case core.MethodSpend:
			fixed = wallet.FixedGasSpend
		}
	case multisig.TemplateAddress1:
		base = multisig.BaseGas1
		switch input.Method {
		case core.MethodSpawn:
			fixed = multisig.FixedGasSpawn1
		case core.MethodSpend:
			fixed = multisig.FixedGasSpend1
		}
	case multisig.TemplateAddress2:
		base = multisig.BaseGas2
		switch input.Method {
		case core.MethodSpawn:
			fixed = multisig.FixedGasSpawn2
		case core.MethodSpend:
			fixed = multisig.FixedGasSpend2
		}
	case multisig.TemplateAddress3:
		base = multisig.BaseGas3
		switch input.Method {
		case core.MethodSpawn:
			fixed = multisig.FixedGasSpawn3
		case core.MethodSpend:
			fixed = multisig.FixedGasSpend3
		}
	}
	return core.ComputeGasCost(base, fixed, input.Raw, e.StorageFactor)
}
