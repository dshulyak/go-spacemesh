package hare

import (
	"go.uber.org/zap/zapcore"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
)

var roundNames = [...]string{"preround", "hardlock", "softlock", "propose", "wait1", "wait2", "commit", "notify"}

type round uint8

func (r round) String() string {
	return roundNames[r]
}

const (
	preround = iota
	hardlock
	softlock
	propose
	wait1
	wait2
	commit
	notify
)

type message struct {
	layer     types.LayerID
	iteration uint8
	round     round
	full      []types.ProposalID // prepare, propose sends full
	reference *types.Hash32      // commit, notify sends reference

	sender    types.NodeID
	malicious bool
}

func (m *message) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddUint32("lid", m.layer.Uint32())
	encoder.AddUint8("iter", m.iteration)
	encoder.AddString("round", m.round.String())
	encoder.AddString("sender", m.sender.ShortString())
	encoder.AddBool("mal", m.malicious)
	if m.full != nil {
		encoder.AddArray("full", zapcore.ArrayMarshalerFunc(func(encoder log.ArrayEncoder) error {
			for _, id := range m.full {
				encoder.AppendString(types.Hash20(id).ShortString())
			}
			return nil
		}))
	} else if m.reference != nil {
		encoder.AddString("ref", m.reference.ShortString())
	}
	return nil
}

type output struct {
	coin       *bool              // set based on preround messages right after preround completes in 0 iteration
	result     []types.ProposalID // set based on notify messages at the start next iteration
	terminated bool               // protocol participates in one more layer after result was computed
}

func (o *output) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddBool("terminated", o.terminated)
	if o.coin != nil {
		encoder.AddBool("coin", *o.coin)
	}
	if o.result != nil {
		encoder.AddArray("result", zapcore.ArrayMarshalerFunc(func(encoder log.ArrayEncoder) error {
			for _, id := range o.result {
				encoder.AppendString(types.Hash20(id).ShortString())
			}
			return nil
		}))
	}
	return nil
}

type validProposal struct {
	iteration uint8
	proposals []types.ProposalID
}

type protocol struct {
	layer          types.LayerID
	iteration      uint8
	round          round
	initial        []types.ProposalID // Si
	locked         *types.Hash32      // Li
	hardLocked     bool
	validProposals map[types.Hash32]validProposal // Vi
	validValues    [4][]types.ProposalID          // 4.3 Protocol Execution. Valid Values
}

func (p *protocol) onMessage(msg *message) error {
	return nil
}

func (p *protocol) active() *message {
	switch p.round {
	case preround:
		return &message{
			layer:     p.layer,
			iteration: p.iteration,
			round:     preround,
			full:      p.initial,
		}
	case propose:
		values := p.validValues[1]
		if p.iteration > 0 {
			// TODO requires thresh gossip
		}
		return &message{
			layer:     p.layer,
			iteration: p.iteration,
			round:     propose,
			full:      values,
		}
	case commit:
		var ref *types.Hash32
		if p.hardLocked && p.locked != nil {
			ref = p.locked
		}
		return &message{
			layer:     p.layer,
			iteration: p.iteration,
			round:     propose,
			reference: ref,
		}
	case notify:
		// TODO requires thresh goss
	}
	return nil
}

func (p *protocol) next() output {
	if p.round == preround && p.iteration == 0 {
		p.round = softlock
	} else if p.round == notify {
		p.round = hardlock
		p.iteration++
	} else {
		p.round++
	}
	return output{}
}
