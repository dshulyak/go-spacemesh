package hare3alt

import (
	"context"
	"errors"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hare/eligibility"
	"go.uber.org/zap"
)

type oracle interface {
	Validate(context.Context, types.LayerID, uint32, int, types.NodeID, types.VrfSignature, uint16) (bool, error)
	CalcEligibility(context.Context, types.LayerID, uint32, int, types.NodeID, types.VrfSignature) (uint16, error)
	Proof(context.Context, types.LayerID, uint32) (types.VrfSignature, error)
}

type LegacyOracle struct {
	log    *zap.Logger
	oracle oracle
	config Config
}

type proof struct {
	vrf           types.VrfSignature
	eligibilities uint16
}

func (lg *LegacyOracle) validate(msg *message) error {
	size := int(lg.config.Committee)
	if msg.round == propose {
		size = int(lg.config.Leaders)
	}
	r := uint32(msg.iter*notify) + uint32(msg.round)
	valid, err := lg.oracle.Validate(context.Background(),
		msg.layer, r,
		size, msg.sender,
		msg.vrf, msg.eligibilities)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid")
	}
	return nil
}

func (lg *LegacyOracle) active(smesher types.NodeID, layer types.LayerID, ir iterround) *proof {
	r := uint32(ir.iter*notify) + uint32(ir.round)
	vrf, err := lg.oracle.Proof(context.Background(), layer, r)
	if err != nil {
		lg.log.Error("failed to compute vrf", zap.Error(err))
		return nil
	}
	size := int(lg.config.Committee)
	if ir.round == propose {
		size = int(lg.config.Leaders)
	}
	elig, err := lg.oracle.CalcEligibility(context.Background(), layer, r, size, smesher, vrf)
	if err != nil {
		if !errors.Is(err, eligibility.ErrNotActive) {
			lg.log.Error("failed to calc eligibilities", zap.Error(err))
		}
		return nil
	}
	return &proof{vrf: vrf, eligibilities: elig}
}
