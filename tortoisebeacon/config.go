package tortoisebeacon

import (
	"math/big"
	"time"

	"github.com/spacemeshos/go-spacemesh/common/types"
)

// Config is the configuration of the Tortoise Beacon.
type Config struct {
	Kappa               uint64        `mapstructure:"tortoise-beacon-kappa"`                  // Security parameter (for calculating ATX threshold)
	Q                   *big.Rat      `mapstructure:"tortoise-beacon-q"`                      // Ratio of dishonest spacetime (for calculating ATX threshold). It should be a string representing a rational number.
	RoundsNumber        types.RoundID `mapstructure:"tortoise-beacon-rounds-number"`          // Amount of rounds in every epoch
	GracePeriodDuration time.Duration `mapstructure:"tortoise-beacon-grace-period-duration"`  // Grace period duration
	ProposalDuration    time.Duration `mapstructure:"tortoise-beacon-proposal-duration"`      // Proposal phase duration
	VotingRoundDuration time.Duration `mapstructure:"tortoise-beacon-voting-round-duration"`  // Voting round duration
	WaitAfterEpochStart time.Duration `mapstructure:"tortoise-beacon-wait-after-epoch-start"` // How long to wait after a new epoch is started.
	Theta               *big.Rat      `mapstructure:"tortoise-beacon-theta"`                  // Ratio of votes for reaching consensus
	VotesLimit          uint64        `mapstructure:"tortoise-beacon-votes-limit"`            // Maximum allowed number of votes to be sent
}

// DefaultConfig returns the default configuration for the tortoise beacon.
func DefaultConfig() Config {
	return Config{
		Kappa:               40,
		Q:                   big.NewRat(1, 3),
		RoundsNumber:        300,
		GracePeriodDuration: 2 * time.Minute,
		ProposalDuration:    2 * time.Minute,
		VotingRoundDuration: 30 * time.Minute,
		WaitAfterEpochStart: 10 * time.Second,
		Theta:               big.NewRat(1, 4),
		VotesLimit:          100, // TODO: around 100, find the calculation in the forum
	}
}

// UnitTestConfig returns the unit test configuration for the tortoise beacon.
func UnitTestConfig() Config {
	return Config{
		Kappa:               400000,
		Q:                   big.NewRat(1, 3),
		RoundsNumber:        2,
		GracePeriodDuration: 20 * time.Millisecond,
		ProposalDuration:    20 * time.Millisecond,
		VotingRoundDuration: 20 * time.Millisecond,
		WaitAfterEpochStart: 1,
		Theta:               big.NewRat(1, 25000),
		VotesLimit:          100,
	}
}

// NodeSimUnitTestConfig returns configuration for the tortoise beacon the unit tests with node simulation .
func NodeSimUnitTestConfig() Config {
	return Config{
		Kappa:               400000,
		Q:                   big.NewRat(1, 3),
		RoundsNumber:        2,
		GracePeriodDuration: 200 * time.Millisecond,
		ProposalDuration:    100 * time.Millisecond,
		VotingRoundDuration: 100 * time.Millisecond,
		WaitAfterEpochStart: 1,
		Theta:               big.NewRat(1, 25000),
		VotesLimit:          100,
	}
}
