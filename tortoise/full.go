package tortoise

import (
	"container/list"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
)

func newFullTortoise(config Config, common *commonState) *full {
	return &full{
		Config:       config,
		commonState:  common,
		votes:        map[types.BallotID]Opinion{},
		base:         map[types.BallotID]types.BallotID{},
		weights:      map[types.BlockID]weight{},
		delayedQueue: list.New(),
	}
}

type full struct {
	Config
	*commonState

	votes        map[types.BallotID]Opinion
	base         map[types.BallotID]types.BallotID
	weights      map[types.BlockID]weight
	delayedQueue *list.List
}

func (f *full) processBallots(ballots []tortoiseBallot) {
	for _, ballot := range ballots {
		f.base[ballot.id] = ballot.base
		f.votes[ballot.id] = ballot.votes
	}
}

func (f *full) processBlocks(blocks []*types.Block) {
	for _, block := range blocks {
		f.weights[block.ID()] = weightFromUint64(0)
	}
}

func (f *full) getVote(logger log.Log, ballot types.BallotID, block types.BlockID) sign {
	sign, exist := f.votes[ballot][block]
	if !exist {
		base, exist := f.base[ballot]
		if !exist {
			return against
		}
		return f.getVote(logger, base, block)
	}
	return sign
}

func (f *full) countVotesFromBallots(logger log.Log, ballotlid types.LayerID, ballots []types.BallotID) {
	var delayed []types.BallotID
	for _, ballot := range ballots {
		if !f.ballotFilter(ballot, ballotlid) {
			delayed = append(delayed, ballot)
			continue
		}
		ballotWeight := f.ballotWeight[ballot]
		for lid := f.verified.Add(1); lid.Before(ballotlid); lid = lid.Add(1) {
			for _, block := range f.blocks[lid] {
				sign := f.getVote(logger, ballot, block)
				current := f.weights[block]

				switch sign {
				case support:
					current = current.add(ballotWeight)
				case against:
					current = current.sub(ballotWeight)
				}
				f.weights[block] = current
			}
		}
	}
	if len(delayed) > 0 {
		f.delayedQueue.PushBack(delayedBallots{
			lid:     ballotlid,
			ballots: delayed,
		})
	}
}

func (f *full) countVotes(logger log.Log, ballotlid types.LayerID) {
	for front := f.delayedQueue.Front(); front != nil; {
		delayed := front.Value.(delayedBallots)
		if f.last.Difference(delayed.lid) <= f.BadBeaconVoteDelayLayers {
			break
		}
		logger.With().Debug("adding weight from delayed ballots", log.Stringer("ballots_layer", delayed.lid))

		f.countVotesFromBallots(logger.WithFields(log.Bool("delayed", true)), delayed.lid, delayed.ballots)

		next := front.Next()
		f.delayedQueue.Remove(front)
		front = next
	}

	f.countVotesFromBallots(logger, ballotlid, f.ballots[ballotlid])
}

func (f *full) verify(logger log.Log) types.LayerID {
	localThreshold := computeLocalThreshold(f.Config, f.epochWeight, f.processed)
	for lid := f.verified.Add(1); lid.Before(f.processed); lid = lid.Add(1) {
		threshold := computeGlobalThreshold(f.Config, f.epochWeight, lid, f.processed)
		threshold = threshold.add(localThreshold)

		llogger := logger.WithFields(
			log.Stringer("candidate_layer", lid),
			log.Stringer("threshold", threshold),
			log.Stringer("local_threshold", localThreshold),
		)

		for _, block := range f.blocks[lid] {
			current := f.weights[block]
			if current.cmp(threshold) == abstain {
				llogger.With().Warning("candidate layer is not verified. block is undecided in full tortoise.",
					log.Stringer("block", block),
					log.Stringer("voting_weight", current),
				)
				return lid.Sub(1)
			}
			f.localOpinion[block] = current.cmp(threshold)
		}

		llogger.With().Info("candidate layer is verified by full tortoise")
	}
	return f.processed.Sub(1)
}

// only ballots with the correct beacon value are considered good ballots and their votes counted by
// verifying tortoise. for ballots with a different beacon values, we count their votes only in self-healing mode
// and if they are old enough (by default using distance equal to an epoch).
func (f *full) ballotFilter(ballotID types.BallotID, ballotlid types.LayerID) bool {
	if _, bad := f.badBeaconBallots[ballotID]; !bad {
		return true
	}
	return f.last.Difference(ballotlid) > f.BadBeaconVoteDelayLayers
}

type delayedBallots struct {
	lid     types.LayerID
	ballots []types.BallotID
}
