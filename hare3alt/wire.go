package hare3alt

import (
	"fmt"

	"github.com/spacemeshos/go-scale"
	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hash"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
)

//go:generate scalegen

type Authenticator struct {
	Smesher   types.NodeID
	Signature types.EdSignature
}

// FullMessage is used to encode messages in preround and propose rounds.
type FullMessage struct {
	Body FullBody
	Auth Authenticator
}

type FullBody struct {
	Round         uint8
	Layer         types.LayerID
	Iteration     uint8
	Eligibilities uint16
	VRF           types.VrfSignature
	Proposals     []types.ProposalID `scale:"max=200"`
}

func (f *FullMessage) toMessage() *message {
	return &message{
		iterround: iterround{
			iter:  f.Body.Iteration,
			round: round(f.Body.Round),
		},
		value:         messageValue{full: f.Body.Proposals},
		layer:         f.Body.Layer,
		vrf:           f.Body.VRF,
		eligibilities: f.Body.Eligibilities,
		sender:        f.Auth.Smesher,
		msgHash:       f.MessageHash(),
		signature:     f.Auth.Signature,
	}
}

func (f *FullMessage) MessageHash() types.Hash32 {
	hash := hash.New()
	_, err := codec.EncodeTo(hash, &f.Body)
	if err != nil {
		panic(err.Error())
	}
	var rst types.Hash32
	hash.Sum(rst[:0])
	return rst
}

func (f *FullMessage) SignedBytes() []byte {
	meta := types.HareMetadata{
		Layer:   f.Body.Layer,
		Round:   uint32(f.Body.Round),
		MsgHash: f.MessageHash(),
	}
	buf, err := codec.Encode(&meta)
	if err != nil {
		panic(err.Error())
	}
	return buf
}

// RefMessage is used to encode messages in commit and notify rounds.
type RefMessage struct {
	Body RefBody
	Auth Authenticator
}

type RefBody struct {
	Round         uint8
	Layer         types.LayerID
	Iteration     uint8
	Eligibilities uint16
	VRF           types.VrfSignature
	Ref           types.Hash32
}

func (r *RefMessage) toMessage() *message {
	return &message{
		iterround: iterround{
			iter:  r.Body.Iteration,
			round: round(r.Body.Round),
		},
		value:         messageValue{reference: &r.Body.Ref},
		layer:         r.Body.Layer,
		vrf:           r.Body.VRF,
		eligibilities: r.Body.Eligibilities,
		sender:        r.Auth.Smesher,
		msgHash:       r.MessageHash(),
		signature:     r.Auth.Signature,
	}
}

func (r *RefMessage) MessageHash() types.Hash32 {
	hash := hash.New()
	_, err := codec.EncodeTo(hash, &r.Body)
	if err != nil {
		panic(err.Error())
	}
	var rst types.Hash32
	hash.Sum(rst[:0])
	return rst
}

func (r *RefMessage) SignedBytes() []byte {
	meta := types.HareMetadata{
		Layer:   r.Body.Layer,
		Round:   uint32(r.Body.Round),
		MsgHash: r.MessageHash(),
	}
	buf, err := codec.Encode(&meta)
	if err != nil {
		panic(err.Error())
	}
	return buf
}

// decodeMessage implements scale enum based on the first field.
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

func encodeWithSignature(msg *message, signer *signing.EdSigner) []byte {
	var enc scale.Encodable
	switch msg.round {
	case preround, propose:
		full := FullMessage{
			Body: FullBody{
				Round:         uint8(msg.round),
				Layer:         msg.layer,
				Iteration:     msg.iter,
				Eligibilities: msg.eligibilities,
				VRF:           msg.vrf,
				Proposals:     msg.value.full,
			},
			Auth: Authenticator{
				Smesher: msg.sender,
			},
		}
		full.Auth.Signature = signer.Sign(signing.HARE, full.SignedBytes())
		enc = &full
	case commit, notify:
		ref := RefMessage{
			Body: RefBody{
				Round:         uint8(msg.round),
				Layer:         msg.layer,
				Iteration:     msg.iter,
				Eligibilities: msg.eligibilities,
				VRF:           msg.vrf,
				Ref:           *msg.value.reference,
			},
			Auth: Authenticator{
				Smesher: msg.sender,
			},
		}
		ref.Auth.Signature = signer.Sign(signing.HARE, ref.SignedBytes())
		enc = &ref
	}
	buf, err := codec.Encode(enc)
	if err != nil {
		panic(err.Error())
	}
	return buf
}
