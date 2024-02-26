package atxsync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/fetch"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/atxsync"
	"github.com/spacemeshos/go-spacemesh/sql/localsql"
	"github.com/spacemeshos/go-spacemesh/system"
)

type fetcher interface {
	PeerEpochInfo(context.Context, p2p.Peer, types.EpochID) (*fetch.EpochData, error)
	SelectBestShuffled(int) []p2p.Peer
	RegisterPeerHashes(p2p.Peer, []types.Hash32)
	system.AtxFetcher
}

type clock interface {
	LayerToTime(types.LayerID) time.Time
}

type Syncer struct {
	logger  *zap.Logger
	cfg     Config
	fetcher fetcher
	clock   clock
	db      sql.Executor
	localdb *localsql.Database
}

type Opt func(*Syncer)

func WithLogger(logger *zap.Logger) Opt {
	return func(s *Syncer) {
		s.logger = logger
	}
}

func DefaultConfig() Config {
	return Config{
		EpochInfoInterval: 20 * time.Minute,
		AtxsBatch:         1000,
		RequestsLimit:     20,
		EpochInfoPeers:    2,
	}
}

type Config struct {
	// EpochInfoInterval between epoch info requests to the network.
	EpochInfoInterval time.Duration `mapstructure:"epoch-info-request-interval"`
	// EpochInfoPeers is the number of peers we will ask for epoch info, every epoch info requests interval.
	EpochInfoPeers int `mapstructure:"epoch-info-peers"`

	// RequestsLimit is the maximum number of requests for single activation.
	//
	// The purpose of it is to prevent peers from advertisting invalid atx and disappearing.
	// Which will make node to ask other peers for invalid atx.
	// It will be reset to 0 once atx advertised again.
	RequestsLimit int `mapstructure:"requests-limit"`

	// AtxsBatch is the maximum number of atxs to sync in a single request.
	AtxsBatch int `mapstructure:"atxs-batch"`
}

func WithConfig(cfg Config) Opt {
	return func(s *Syncer) {
		s.cfg = cfg
	}
}

func New(fetcher fetcher, clock clock, db sql.Executor, localdb *localsql.Database, opts ...Opt) *Syncer {
	s := &Syncer{
		logger:  zap.NewNop(),
		cfg:     DefaultConfig(),
		fetcher: fetcher,
		db:      db,
		localdb: localdb,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Syncer) downloadEpochInfo(
	ctx context.Context,
	epoch types.EpochID,
	immediate bool,
	updates chan map[types.ATXID]int,
) error {
	for {
		if !immediate {
			select {
			case <-ctx.Done():
				return nil
			// TODO this has to be randomized so that different peers don't request at the same time
			case <-time.After(s.cfg.EpochInfoInterval):
			}
		}
		peers := s.fetcher.SelectBestShuffled(s.cfg.EpochInfoPeers)
		if len(peers) == 0 {
			return fmt.Errorf("no peers available")
		}
		// do not run it concurrently, epoch info is large and will continue to grow
		for _, peer := range peers {
			epochData, err := s.fetcher.PeerEpochInfo(ctx, peer, epoch)
			if err != nil || epochData == nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				s.logger.Warn("failed to get epoch info from peer",
					epoch.Field().Zap(),
					zap.String("peer", peer.String()),
					zap.Error(err),
				)
				continue
			}
			s.logger.Info("got epoch info from peer",
				epoch.Field().Zap(),
				zap.String("peer", peer.String()),
				zap.Int("atxs", len(epochData.AtxIDs)),
			)
			// after first success switch immediate off
			immediate = false
			if err := atxsync.SaveRequestTime(s.localdb, epoch, time.Now()); err != nil {
				return fmt.Errorf("failed to save request time: %w", err)
			}
			s.fetcher.RegisterPeerHashes(peer, types.ATXIDsToHashes(epochData.AtxIDs))
			state := map[types.ATXID]int{}
			for _, atx := range epochData.AtxIDs {
				state[atx] = 0
			}
			select {
			case <-ctx.Done():
				return nil
			case updates <- state:
			}
		}
		return nil
	}

}

func (s *Syncer) downloadAtxs(
	ctx context.Context,
	epoch types.EpochID,
	state map[types.ATXID]int,
	updates chan map[types.ATXID]int,
) error {
	batch := make([]types.ATXID, 0, s.cfg.AtxsBatch)
	batch = batch[:0]
	var (
		lackOfProgress       = 0
		progressTimestamp    time.Time
		downloaded           = map[types.ATXID]bool{}
		previouslyDownloaded = 0
		start                = time.Now()
	)

	for {
		// if state is empty we want to block on updates
		if len(state) == 0 {
			select {
			case <-ctx.Done():
				return nil
			case update := <-updates:
				for atx, count := range update {
					state[atx] = count
				}
			}
		} else {
			// otherwise we want to check if we received new updates, but not block on them
			select {
			case <-ctx.Done():
				return nil
			case update := <-updates:
				for atx, count := range update {
					state[atx] = count
				}
			default:
			}
		}

		for atx, requests := range state {
			if downloaded[atx] {
				continue
			}
			exists, err := atxs.Has(s.db, atx)
			if err != nil {
				return err
			}
			if exists {
				downloaded[atx] = true
				continue
			}
			if requests >= s.cfg.RequestsLimit {
				continue
			}
			batch = append(batch, atx)
			if len(batch) == cap(batch) {
				break
			}
		}
		if len(downloaded) == previouslyDownloaded {
			lackOfProgress++
		}
		// report progress every 10% or if downloaded more than requested in a batch
		if progress := float64(len(downloaded) - previouslyDownloaded); progress/float64(len(state)) > 0.1 ||
			progress > float64(s.cfg.AtxsBatch) {
			rate := float64(0)
			if previouslyDownloaded != 0 {
				rate = progress / time.Since(progressTimestamp).Seconds()
			}
			previouslyDownloaded = len(downloaded)
			progressTimestamp = time.Now()
			lackOfProgress = 0
			s.logger.Info(
				"atx sync progress",
				epoch.Field().Zap(),
				zap.Int("downloaded", len(downloaded)),
				zap.Int("total", len(state)),
				zap.Float64("rate per sec", rate),
			)
		}
		if lackOfProgress == 10 {
			lackOfProgress = 0
			s.logger.Warn("no progress in atx sync",
				epoch.Field().Zap(),
				zap.Int("downloaded", len(downloaded)),
				zap.Int("total", len(state)),
			)
		}
		// condition for termination
		if len(batch) == 0 && len(state) != 0 {
			s.logger.Info(
				"atx sync for epoch is finished",
				epoch.Field().Zap(),
				zap.Int("downloaded", len(downloaded)),
				zap.Int("total", len(state)),
				zap.Int("unavailable", len(state)-len(downloaded)),
				zap.Duration("duration", time.Since(start)),
			)
			return nil
		}
		if err := s.fetcher.GetAtxs(ctx, batch); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			s.logger.Debug("failed to download atxs", zap.Error(err))
			// TODO should be updated only if peer failed to provide atx
			// for _, atx := range batch {
			// 	state[atx]++
			// }
		}

		if err := s.localdb.WithTxImmediate(ctx, func(tx *sql.Tx) error {
			return atxsync.SaveSyncState(tx, epoch, state)
		}); err != nil {
			return fmt.Errorf("failed to persist state for epoch %v: %w", epoch, err)
		}
		batch = batch[:0]
	}
}

func (s *Syncer) Download(ctx context.Context, epoch types.EpochID) error {
	epochStart := s.clock.LayerToTime(epoch.FirstLayer())
	s.logger.Info("scheduled atxs downloading",
		epoch.Field().Zap(),
		zap.Time("epoch start", epochStart),
	)

	state, err := atxsync.GetSyncState(s.localdb, epoch)
	if err != nil {
		return fmt.Errorf("failed to get state for epoch %v: %w", epoch, err)
	}
	// immediate specifies if we should poll epoch info immediately or wait for the next scheduled time
	immediate := len(state) == 0
	lastSuccess, err := atxsync.GetRequestTime(s.localdb, epoch)
	if err != nil && !errors.Is(err, sql.ErrNotFound) {
		return fmt.Errorf("failed to get last request time for epoch %v: %w", epoch, err)
	} else if errors.Is(err, sql.ErrNotFound) {
		immediate = true
	} else {
		immediate = immediate || lastSuccess.Sub(epochStart) > 2*s.cfg.EpochInfoInterval
	}

	ctx, cancel := context.WithCancel(ctx)
	eg, ctx := errgroup.WithContext(ctx)
	updates := make(chan map[types.ATXID]int, s.cfg.EpochInfoPeers)
	if state == nil {
		state = map[types.ATXID]int{}
	} else {
		updates <- state
	}

	eg.Go(func() error {
		return s.downloadEpochInfo(ctx, epoch, immediate, updates)
	})

	// TODO improve termination when atxs are unavailable
	// this requires propagation of the errors from the fetcher
	eg.Go(func() error {
		err := s.downloadAtxs(ctx, epoch, state, updates)
		cancel()
		return err
	})

	// termination should be realized by cancelling the context. there are two conditions:
	// - downloaded succesfully epoch info not more than 2*EpochInfoInterval ago
	// - download all atxs in epoch info or atxs are unavailable.
	//   atx is unavailable if it was requested more than RequestsLimit times

	return eg.Wait()
}
