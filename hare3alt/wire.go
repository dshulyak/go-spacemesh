package hare3alt

import (
	"fmt"

	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
)

//go:generate scalegen

type Common struct {
	// NOTE(dshulyak) unlike messages that we have to keep in state
	// hare is completely ephemeral and versioned using p2p protocol string
	Layer         types.LayerID
	Iteration     uint8
	Eligibilities uint16
	VRF           types.VrfSignature
	Smesher       types.NodeID
	Signature     types.EdSignature
}

func (c *Common) toMessage() *message {
	return &message{
		iterround: iterround{
			iter: c.Iteration,
		},
		layer:         c.Layer,
		sender:        c.Smesher,
		vrf:           c.VRF,
		eligibilities: c.Eligibilities,
		signature:     c.Signature,
	}
}

// FullMessage is used to encode messages in preround and propose rounds.
type FullMessage struct {
	Round     uint8
	Proposals []types.ProposalID `scale:"max=200"`
	Common    Common
}

func (f *FullMessage) toMessage() *message {
	msg := f.Common.toMessage()
	msg.round = round(f.Round)
	msg.value.full = f.Proposals
	return msg
}

// RefMessage is used to encode messages in commit and notify rounds.
type RefMessage struct {
	Round     uint8
	Reference types.Hash32
	Common    Common
}

func (r *RefMessage) toMessage() *message {
	msg := r.Common.toMessage()
	msg.round = round(r.Round)
	msg.value.reference = &r.Reference
	return msg
}

func decodeMessage(msg []byte) (*message, error) {
	if len(msg) == 0 {
		return nil, fmt.Errorf("%w: empty message", pubsub.ErrValidationReject)
	}
	switch round(msg[0]) {
	case preround, propose:
		var full FullMessage
		if err := codec.Decode(msg, &full); err != nil {
			return nil, fmt.Errorf("%w: decoding error %s",
				pubsub.ErrValidationReject, err.Error())
		}
		return full.toMessage(), nil
	case commit, notify:
		var ref RefMessage
		if err := codec.Decode(msg, &ref); err != nil {
			return nil, fmt.Errorf("%w: decoding error %s",
				pubsub.ErrValidationReject, err.Error())
		}
		return ref.toMessage(), nil
	}
	return nil, fmt.Errorf("%w: unknown message round %d", pubsub.ErrValidationReject, msg[0])
}
