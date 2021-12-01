package system

import (
	"context"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

// Tortoise is an interface for tortoise protocol implementation.
type Tortoise interface {
	HandleIncomingLayer(context.Context, types.LayerID) (types.LayerID, types.LayerID, bool)
	BaseBlock(context.Context) (types.BlockID, [][]types.BlockID, error)
}
