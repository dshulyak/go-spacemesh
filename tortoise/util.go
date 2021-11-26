package tortoise

import (
	"math/big"
	"sync"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
)

var (
	// correction vectors type.
	support = vec{Sign: 1}
	against = vec{Sign: -1}
	abstain = vec{Sign: 0}
)

type vec struct {
	Sign    int8
	Flushed bool
}

func equalVotes(i, j vec) bool {
	return i.Sign == j.Sign
}

func (a vec) String() string {
	switch a.Sign {
	case 1:
		return "support"
	case -1:
		return "against"
	case 0:
		return "abstain"
	default:
		panic("sign should be 0/-1/1")
	}
}

// Some methods of *big.Rat change its internal data without mutexes, which causes data races.
var thetaMu sync.Mutex

// calculateOpinionWithThreshold computes opinion vector (support, against, abstain) based on the vote weight
// theta, and expected vote weight.
func calculateOpinionWithThreshold(logger log.Log, vote *big.Float, theta *big.Rat, weight *big.Float) vec {
	thetaMu.Lock()
	defer thetaMu.Unlock()

	threshold := new(big.Float).SetInt(theta.Num())
	threshold.Mul(threshold, weight)
	threshold.Quo(threshold, new(big.Float).SetInt(theta.Denom()))

	logger.With().Debug("threshold opinion",
		log.Stringer("vote", vote),
		log.Stringer("theta", theta),
		log.Stringer("expected_weight", weight),
		log.Stringer("threshold", threshold),
	)

	switch vote.Sign() {
	case 1:
		if vote.Cmp(threshold) == 1 {
			return support
		}
	case -1:
		if vote.Abs(vote).Cmp(threshold) == 1 {
			return against
		}
	}
	// either vote is 0 or it didn't cross the threshold
	return abstain
}

// Opinion is opinions on other blocks.
type Opinion map[types.BlockID]vec

func blockMapToArray(m map[types.BlockID]struct{}) []types.BlockID {
	arr := make([]types.BlockID, 0, len(m))
	for b := range m {
		arr = append(arr, b)
	}
	return arr
}

func addVoteToSum(vote vec, sum, weight *big.Float) *big.Float {
	adjusted := weight
	if vote == against {
		// copy is needed only if we modify sign
		adjusted = new(big.Float).Mul(weight, big.NewFloat(-1))
	}
	if vote != abstain {
		sum = sum.Add(sum, adjusted)
	}
	return sum
}
