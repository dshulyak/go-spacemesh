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

type grade uint8

const (
	grade0 grade = iota
	grade1
	grade2
	grade3
	grade4
	grade5
)

type iterround struct {
	iter  uint8
	round round
}

func (ir iterround) since(since iterround) int {
	return int(ir.iter*notify+uint8(ir.round)) - int(since.iter*notify+uint8(since.round))
}

type message struct {
	iterround
	full      []types.ProposalID // prepare, propose sends full
	reference *types.Hash32      // commit, notify sends reference

	sender    types.NodeID
	malicious bool

	vrf           types.VrfSignature
	eligibilities uint16

	// part of hare metadata required for malfeasence proof
	msgHash   types.Hash32
	signature types.EdSignature
}

type messageKey struct {
	iterround
	sender types.NodeID
}

func (m *message) key() messageKey {
	return messageKey{
		sender:    m.sender,
		iterround: m.iterround,
	}
}

func (m *message) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddUint8("iter", m.iter)
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
	coin       *bool              // set based on preround messages right after preround completes in 0 iter
	result     []types.ProposalID // set based on notify messages at the start of next iter
	terminated bool               // protocol participates in one more iteration after outputing result
	message    *message
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
	if o.message != nil {
		encoder.AddObject("msg", o.message)
	}
	return nil
}

type validProposal struct {
	iter      uint8
	proposals []types.ProposalID
}

type gradedProposals struct {
	values [6][]types.ProposalID // the length is set so that 5 is a valid index
}

func (g *gradedProposals) set(gr grade, values []types.ProposalID) {
	g.values[gr] = values
}

func (g *gradedProposals) get(gr grade) []types.ProposalID {
	return g.values[gr]
}

type protocol struct {
	iterround
	coin            *types.VrfSignature // smallest vrf from preround messages. not a part of paper
	initial         []types.ProposalID  // Si
	locked          *types.Hash32       // Li
	hardLocked      bool
	validProposals  map[types.Hash32]validProposal // Vi
	gradedProposals gradedProposals                // valid values in 4.3
	gradedGossip    gradedGossip
	gradecast       gradecast
	thresholdGossip thresholdGossip
}

func (p *protocol) onMessage(msg *message) (bool, *types.HareProof) {
	gossip, equivocation := p.gradedGossip.receive(msg)
	if !gossip {
		return false, equivocation
	}
	if msg.round == propose {
		p.gradecast.add(grade3, p.iterround, msg)
	} else {
		p.thresholdGossip.add(grade5, p.iterround, msg)
	}
	if msg.round == preround {
		if p.coin != nil {
			if msg.vrf.Cmp(p.coin) == -1 {
				p.coin = &msg.vrf
			}
		} else {
			p.coin = &msg.vrf
		}
	}
	return gossip, equivocation
}

func (p *protocol) next(active bool) output {
	out := output{}
	if p.iter == 0 && p.round >= softlock && p.round <= wait2 {
		p.gradedProposals.set(grade5-grade(p.round-2), nil)
	}
	if p.round == preround && active {
		out.message = &message{
			iterround: p.iterround,
			full:      p.initial,
		}
	} else if p.round == hardlock && p.iter > 0 {

	} else if p.round == softlock && p.iter > 0 {

	} else if p.round == propose && active {
		values := p.gradedProposals.get(grade4)
		if p.iter > 0 {
		}
		out.message = &message{
			iterround: p.iterround,
			full:      values,
		}
	} else if p.round == commit && active {
		var ref *types.Hash32
		if p.hardLocked && p.locked != nil {
			ref = p.locked
		}
		out.message = &message{
			iterround: p.iterround,
			reference: ref,
		}
	} else if p.round == notify && active {
		out.message = &message{
			iterround: p.iterround,
			reference: nil,
		}
	}
	if p.round == preround && p.iter == 0 {
		p.round = softlock
		if p.coin != nil {
			coin := p.coin.LSB() != 0
			out.coin = &coin
		}
	} else if p.round == notify {
		p.round = hardlock
		p.iter++
	} else {
		p.round++
	}
	return out
}

type gradedGossip struct {
	state map[messageKey]*message
}

func (g *gradedGossip) receive(msg *message) (bool, *types.HareProof) {
	if g.state == nil {
		g.state = map[messageKey]*message{}
	}
	other, exist := g.state[msg.key()]
	if exist {
		if other.msgHash != msg.msgHash && !other.malicious {
			other.malicious = true
			return true, &types.HareProof{} // TODO construct proof
		}
		return false, nil
	}
	g.state[msg.key()] = msg
	return true, nil
}

type gradecasted struct {
	grade     grade
	received  iterround
	malicious bool
	values    []types.ProposalID
}

func (g *gradecasted) ggrade(current iterround) grade {
	switch {
	case !g.malicious && g.grade == grade3 && current.since(g.received) <= 1:
		return grade2
	case !g.malicious && g.grade >= grade2 && current.since(g.received) <= 2:
		return grade1
	default:
		return grade0
	}
}

type gradecast struct {
	state map[messageKey]*gradecasted
}

func (g *gradecast) add(grade grade, current iterround, msg *message) {
	if g.state != nil {
		g.state = map[messageKey]*gradecasted{}
	}
	if current.since(msg.iterround) > 3 {
		return
	}
	gc := gradecasted{
		grade:     grade,
		received:  current,
		malicious: msg.malicious,
		values:    msg.full,
	}
	other, exist := g.state[msg.key()]
	if !exist {
		g.state[msg.key()] = &gc
		return
	}
	if other.malicious {
		return
	}
	switch {
	case other.ggrade(current) == grade2 && current.since(gc.received) <= 3:
		other.malicious = true
	case other.ggrade(current) == grade1 && current.since(gc.received) <= 2:
		other.malicious = true
	}
}

type gset struct {
	values []types.ProposalID
	grade  grade
}

func (g *gradecast) filter(filter iterround) []gset {
	var rst []gset
	for key, value := range g.state {
		if key.iterround == filter {
			if grade := value.ggrade(filter); grade > 0 {
				rst = append(rst, gset{
					grade:  grade,
					values: value.values,
				})
			}
		}
	}
	return rst
}

type thresholdGossip struct {
}

func (t *thresholdGossip) add(grade grade, current iterround, msg *message) {
	if current.since(msg.iterround) > 5 {
		return
	}
}
