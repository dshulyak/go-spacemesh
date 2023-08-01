package hare3alt

import (
	"testing"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log/logtest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func castIds(strings ...string) []types.ProposalID {
	ids := []types.ProposalID{}
	for _, p := range strings {
		var id types.ProposalID
		copy(id[:], p)
		ids = append(ids, id)
	}
	return ids
}

type tinput struct {
	input
	expect *response
}

func (t *tinput) ensureMsg() {
	if t.Message == nil {
		t.Message = &Message{}
	}
}

func (t *tinput) ensureResponse() {
	if t.expect != nil {
		t.expect = &response{}
	}
}

func (t *tinput) round(r Round) *tinput {
	t.ensureMsg()
	t.Round = r
	return t
}

func (t *tinput) iter(i uint8) *tinput {
	t.ensureMsg()
	t.Iter = i
	return t
}

func (t *tinput) proposals(proposals ...string) *tinput {
	t.ensureMsg()
	t.Value.Proposals = castIds(proposals...)
	return t
}

func (t *tinput) ref(proposals ...string) *tinput {
	t.ensureMsg()
	hs := types.CalcProposalHash32Presorted(castIds(proposals...), nil)
	t.Value.Reference = &hs
	return t
}

func (t *tinput) vrf(vrf ...byte) *tinput {
	t.ensureMsg()
	copy(t.Eligibility.Proof[:], vrf)
	return t
}

func (t *tinput) vrfcount(c uint16) *tinput {
	t.ensureMsg()
	t.Eligibility.Count = c
	return t
}

func (t *tinput) sender(name string) *tinput {
	t.ensureMsg()
	copy(t.Sender[:], name)
	return t
}

func (t *tinput) mshHash(h string) *tinput {
	copy(t.input.msgHash[:], h)
	return t
}

func (t *tinput) malicious() *tinput {
	t.input.malicious = true
	return t
}

func (t *tinput) gossip() *tinput {
	t.ensureResponse()
	t.expect.gossip = true
	return t
}

func (t *tinput) g(g grade) *tinput {
	t.grade = g
	return t
}

type toutput struct {
	act bool
	output
}

func (t *toutput) ensureMsg() {
	if t.message == nil {
		t.message = &Message{}
	}
}

func (t *toutput) active() *toutput {
	t.act = true
	return t
}

func (t *toutput) round(r Round) *toutput {
	t.ensureMsg()
	t.message.Round = r
	return t
}

func (t *toutput) iter(i uint8) *toutput {
	t.ensureMsg()
	t.message.Iter = i
	return t
}

func (t *toutput) proposals(proposals ...string) *toutput {
	t.ensureMsg()
	t.message.Value.Proposals = castIds(proposals...)
	return t
}

func (t *toutput) ref(proposals ...string) *toutput {
	t.ensureMsg()
	hs := types.CalcProposalHash32Presorted(castIds(proposals...), nil)
	t.message.Value.Reference = &hs
	return t
}

func (t *toutput) terminated() *toutput {
	t.output.terminated = true
	return t
}

func (t *toutput) coin(c bool) *toutput {
	t.output.coin = &c
	return t
}

func (t *toutput) result(proposals ...string) *toutput {
	t.output.result = castIds(proposals...)
	return t
}

type setup struct {
	threshold uint16
	proposals []types.ProposalID
}

func (s *setup) thresh(v uint16) *setup {
	s.threshold = v
	return s
}

func (s *setup) initial(proposals ...string) *setup {
	s.proposals = castIds(proposals...)
	return s
}

type testCase struct {
	desc  string
	steps []any
}

func gen(desc string, steps ...any) testCase {
	return testCase{desc: desc, steps: steps}
}

func TestProtocol(t *testing.T) {
	for _, tc := range []testCase{
		gen("sanity", // simplest e2e protocol run
			new(setup).thresh(10).initial("a", "b"),
			new(toutput).active().round(preround).proposals("a", "b"),
			new(tinput).sender("1").round(preround).proposals("b", "a").vrfcount(3).g(grade5),
			new(tinput).sender("2").round(preround).proposals("a", "c").vrfcount(9).g(grade5),
			new(tinput).sender("3").round(preround).proposals("c").vrfcount(6).g(grade5),
			new(toutput).coin(false),
			new(toutput).active().round(propose).proposals("a", "c"),
			new(tinput).sender("1").round(propose).proposals("a", "c").g(grade3).vrf(2),
			new(tinput).sender("2").round(propose).proposals("b", "d").g(grade3).vrf(1),
			new(toutput),
			new(toutput),
			new(toutput).active().round(commit).ref("a", "c"),
			new(tinput).sender("1").round(commit).ref("a", "c").vrfcount(4).g(grade5),
			new(tinput).sender("2").round(commit).ref("a", "c").vrfcount(8).g(grade5),
			new(toutput).active().round(notify).ref("a", "c"),
			new(tinput).sender("1").round(notify).ref("a", "c").vrfcount(5).g(grade5),
			new(tinput).sender("2").round(notify).ref("a", "c").vrfcount(6).g(grade5),
			new(toutput).result("a", "c"),
			new(toutput), // compress this somehow
			new(toutput),
			new(toutput),
			new(toutput),
			new(toutput),
			new(toutput),
			new(toutput).terminated(),
		),
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			var (
				proto  *protocol
				logger = logtest.New(t).Zap()
			)
			for i, step := range tc.steps {
				if i != 0 && proto == nil {
					require.FailNow(t, "step with setup should be the first one")
				}
				switch casted := step.(type) {
				case *setup:
					proto = newProtocol(casted.proposals, casted.threshold)
				case *tinput:
					logger.Debug("input", zap.Int("i", i), zap.Inline(casted))
					gossip, _ := proto.onMessage(&casted.input)
					if casted.expect != nil {
						require.Equal(t, casted.expect.gossip, gossip, "%d", i)
					}
				case *toutput:
					before := proto.Round
					require.Equal(t, casted.output, proto.next(casted.act), "%d", i)
					logger.Debug("output",
						zap.Int("i", i),
						zap.Inline(casted),
						zap.Stringer("before", before),
						zap.Stringer("after", proto.Round),
					)
				}
			}
		})
	}
}
