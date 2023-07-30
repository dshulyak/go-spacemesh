package hare3alt

import (
	"go.uber.org/zap/zapcore"

	"github.com/spacemeshos/go-spacemesh/codec"
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

func isSubset(s, v []types.ProposalID) bool {
	set := map[types.ProposalID]struct{}{}
	for _, id := range v {
		set[id] = struct{}{}
	}
	for _, id := range s {
		if _, exist := set[id]; !exist {
			return false
		}
	}
	return true
}

func toHash(proposals []types.ProposalID) types.Hash32 {
	return types.CalcProposalHash32Presorted(proposals, nil)
}

type iterround struct {
	iter  uint8
	round round
}

func (ir iterround) since(since iterround) int {
	return int(ir.iter*notify+uint8(ir.round)) - int(since.iter*notify+uint8(since.round))
}

type messageValue struct {
	full      []types.ProposalID // prepare, propose sends full
	reference *types.Hash32      // commit, notify sends reference
}

type message struct {
	iterround
	layer types.LayerID
	value messageValue

	sender    types.NodeID
	malicious bool

	vrf           types.VrfSignature
	eligibilities uint16

	// part of hare metadata required for malfeasence proof
	msgHash   types.Hash32
	signature types.EdSignature
}

func (m *message) signedBytes() []byte {
	meta := types.HareMetadata{
		Layer:   m.layer,
		Round:   uint32(m.round),
		MsgHash: m.msgHash,
	}
	buf, err := codec.Encode(&meta)
	if err != nil {
		panic(err.Error())
	}
	return buf
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
	encoder.AddBool("malicious", m.malicious)
	if m.value.full != nil {
		encoder.AddArray("full", zapcore.ArrayMarshalerFunc(func(encoder log.ArrayEncoder) error {
			for _, id := range m.value.full {
				encoder.AppendString(types.Hash20(id).ShortString())
			}
			return nil
		}))
	} else if m.value.reference != nil {
		encoder.AddString("ref", m.value.reference.ShortString())
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
	values [6][]types.ProposalID // the length is set to 6 so that 5 is a valid index
}

func (g *gradedProposals) set(gr grade, values []types.ProposalID) {
	g.values[gr] = values
}

func (g *gradedProposals) get(gr grade) []types.ProposalID {
	return g.values[gr]
}

func newProtocol(initial []types.ProposalID, threshold uint16) *protocol {
	return &protocol{
		initial:        initial,
		validProposals: map[types.Hash32]validProposal{},
		gradedGossip:   gradedGossip{state: map[messageKey]*message{}},
		gradecast:      gradecast{state: map[messageKey]*gradecasted{}},
		thresholdGossip: thresholdGossip{
			threshold: threshold,
			state:     map[messageKey]*votes{},
		},
	}
}

type protocol struct {
	iterround
	coin            *types.VrfSignature // smallest vrf from preround messages. not a part of paper
	initial         []types.ProposalID  // Si
	result          *types.Hash32       // set after Round 6. Case 1
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
	// gradecast and thresholdGossip should never be called with non-equivocating duplicates
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

func (p *protocol) thresholdGossipMessage(ir iterround, grade grade) (*types.Hash32, []types.ProposalID) {
	for _, ref := range p.thresholdGossip.filterref(ir, grade) {
		valid, exist := p.validProposals[ref]
		if !exist || valid.iter > ir.iter {
			continue
		}
		return &ref, valid.proposals
	}
	return nil, nil
}

func (p *protocol) thresholdGossipCommit(iter uint8, grade grade) (*types.Hash32, []types.ProposalID) {
	return p.thresholdGossipMessage(iterround{iter: iter, round: commit}, grade)
}

func (p *protocol) thresholdGossipExists(iter uint8, grade grade, match types.Hash32) bool {
	for _, ref := range p.thresholdGossip.filterref(iterround{iter: iter, round: commit}, grade) {
		if ref == match {
			return true
		}
	}
	return false
}

func (p *protocol) execution(out *output, active bool) {
	// code below aims to look similar to 4.3 Protocol Execution
	// NOTE(dshulyak) please keep code in this method forward only
	if p.iter == 0 && p.round >= softlock && p.round <= wait2 {
		// -1 - skipped hardlock round in iter 0
		// -1 - implementation rounds starts from 0
		g := grade5 - grade(p.round-2)
		p.gradedProposals.set(g, p.thresholdGossip.filter(p.iterround, g))
	}
	if p.round == preround && active {
		out.message = &message{
			iterround: p.iterround,
			value:     messageValue{full: p.initial},
		}
	} else if p.round == hardlock && p.iter > 0 {
		if p.result != nil {
			out.terminated = true
		}
		ref, values := p.thresholdGossipMessage(iterround{iter: p.iter - 1, round: notify}, grade5)
		if ref != nil {
			p.result = ref
			out.result = values
		}
		if ref, _ := p.thresholdGossipCommit(p.iter-1, grade4); ref != nil {
			p.locked = ref
			p.hardLocked = true
		}
	} else if p.round == softlock && p.iter > 0 {
		if ref, _ := p.thresholdGossipCommit(p.iter-1, grade3); ref != nil {
			p.locked = ref
		}
	} else if p.round == propose && active {
		values := p.gradedProposals.get(grade4)
		if p.iter > 0 {
			ref, overwrite := p.thresholdGossipCommit(p.iter-1, grade2)
			if ref != nil {
				values = overwrite
			}
		}
		out.message = &message{
			iterround: p.iterround,
			value:     messageValue{full: values},
		}
	} else if p.round == commit && p.result == nil {
		proposed := p.gradecast.filter(iterround{iter: p.iter, round: propose})
		for _, graded := range proposed {
			if graded.grade < grade1 || !isSubset(graded.values, p.gradedProposals.get(grade2)) {
				continue
			}
			p.validProposals[toHash(graded.values)] = validProposal{
				iter:      p.iter,
				proposals: graded.values,
			}
		}
		if active {
			var ref *types.Hash32
			if p.hardLocked && p.locked != nil {
				ref = p.locked
			} else {
				for _, graded := range proposed {
					id := toHash(graded.values)
					// condition (c)
					if proposal, exist := p.validProposals[id]; !exist || proposal.iter > p.iter {
						continue
					}
					// condition (d) IsLeader is implicit, propose won't pass eligibility check
					// condition (e)
					if graded.grade != grade2 {
						continue
					}
					// condition (f)
					if !isSubset(graded.values, p.gradedProposals.get(grade3)) {
						continue
					}
					// condition (g)
					if !isSubset(p.gradedProposals.get(grade5), graded.values) ||
						!p.thresholdGossipExists(p.iter-1, grade1, id) {
						continue
					}
					// condition (h)
					if p.locked != nil && *p.locked != id {
						continue
					}
					ref = &id
				}
			}
			if ref != nil {
				out.message = &message{
					iterround: p.iterround,
					value:     messageValue{reference: ref},
				}
			}
		}
	} else if p.round == notify && active {
		ref := p.result
		if ref == nil {
			ref, _ = p.thresholdGossipCommit(p.iter, grade5)
		}
		if ref != nil {
			out.message = &message{
				iterround: p.iterround,
				value:     messageValue{reference: ref},
			}
		}
	}
}

func (p *protocol) next(active bool) output {
	out := output{}
	p.execution(&out, active)
	if p.round == softlock && p.iter == 0 && p.coin != nil {
		coin := p.coin.LSB() != 0
		out.coin = &coin
	}
	if p.round == preround && p.iter == 0 {
		p.round = softlock // skips hardlock
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
	other, exist := g.state[msg.key()]
	if exist {
		if other.msgHash != msg.msgHash && !other.malicious {
			other.malicious = true
			return true, toProof(other, msg)
		}
		return false, nil
	}
	g.state[msg.key()] = msg
	return true, nil
}

func toProof(first, second *message) *types.HareProof {
	return &types.HareProof{
		Messages: [2]types.HareProofMsg{
			{
				InnerMsg: types.HareMetadata{
					Layer:   first.layer,
					Round:   uint32(first.round),
					MsgHash: first.msgHash,
				},
				SmesherID: first.sender,
				Signature: first.signature,
			},
			{
				InnerMsg: types.HareMetadata{
					Layer:   second.layer,
					Round:   uint32(second.round),
					MsgHash: second.msgHash,
				},
				SmesherID: second.sender,
				Signature: second.signature,
			},
		},
	}
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
	if current.since(msg.iterround) > 3 {
		return
	}
	gc := gradecasted{
		grade:     grade,
		received:  current,
		malicious: msg.malicious,
		values:    msg.value.full,
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

type votes struct {
	grade         grade
	eligibilities uint16
	malicious     bool
	value         messageValue
}
type thresholdGossip struct {
	threshold uint16
	state     map[messageKey]*votes
}

func (t *thresholdGossip) add(grade grade, current iterround, msg *message) {
	if current.since(msg.iterround) > 5 {
		return
	}
	other, exist := t.state[msg.key()]
	if !exist {
		t.state[msg.key()] = &votes{
			grade:         grade,
			eligibilities: msg.eligibilities,
			malicious:     msg.malicious,
			value:         msg.value,
		}
	} else {
		other.malicious = true
	}
}

// filter returns union of sorted proposals received
// in the given round with minimal specified grade.
func (t *thresholdGossip) filter(filter iterround, grade grade) []types.ProposalID {
	all := map[types.ProposalID]uint16{}
	good := map[types.ProposalID]struct{}{}
	for key, value := range t.state {
		if key.iterround == filter && value.grade >= grade {
			for _, id := range value.value.full {
				all[id] += value.eligibilities
				if !value.malicious {
					good[id] = struct{}{}
				}
			}
		}
	}
	var rst []types.ProposalID
	for id := range good {
		if all[id] >= t.threshold {
			rst = append(rst, id)
		}
	}
	types.SortProposalIDs(rst)
	return rst
}

// filterred returns all references to proposals in the given round with minimal grade.
func (t *thresholdGossip) filterref(filter iterround, grade grade) []types.Hash32 {
	all := map[types.Hash32]uint16{}
	good := map[types.Hash32]struct{}{}
	for key, value := range t.state {
		if key.iterround == filter && value.grade >= grade {
			// nil should not be valid in this codepath
			// this is enforced by correctly decoded messages
			id := *value.value.reference
			all[id] += value.eligibilities
			if !value.malicious {
				good[id] = struct{}{}
			}
		}
	}
	var rst []types.Hash32
	for id := range good {
		if all[id] >= t.threshold {
			rst = append(rst, id)
		}
	}
	return rst
}
