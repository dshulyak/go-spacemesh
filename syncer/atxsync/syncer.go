package atxsync

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/fetch"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/atxsync"
	"github.com/spacemeshos/go-spacemesh/sql/localsql"
	"github.com/spacemeshos/go-spacemesh/system"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type fetcher interface {
	PeerEpochInfo(context.Context, p2p.Peer, types.EpochID) (*fetch.EpochData, error)
	SelectBestShuffled(int) []p2p.Peer
	RegisterPeerHashes(p2p.Peer, []types.Hash32)
	system.AtxFetcher
}

type Syncer struct {
	logger  *zap.Logger
	cfg     Config
	fetcher fetcher
	db      sql.Executor
	localdb *localsql.Database
}

type Opt func(*Syncer)

func WithLogger(logger *zap.Logger) Opt {
	return func(s *Syncer) {
		s.logger = logger
	}
}

type Config struct {
	// EpochInfoRequestInterval between epoch info requests to the network.
	EpochInfoRequestInterval time.Duration
	// RequestsLimit is the maximum number of requests for single activation.
	//
	// The purpose of it is to prevent peers from advertisting invalid atx and disappearing.
	// Which will make node to ask other peers for invalid atx.
	// It will be reset to 0 once atx advertised again.
	RequestsLimit int
	// Batch is the maximum number of atxs to sync in a single request.
	Batch int
	Peers int
}

func WithConfig(cfg Config) Opt {
	return func(s *Syncer) {
		s.cfg = cfg
	}
}

func New(fetcher fetcher, db sql.Executor, localdb *localsql.Database, opts ...Opt) *Syncer {
	s := &Syncer{
		logger: zap.NewNop(),
		cfg: Config{
			EpochInfoRequestInterval: time.Hour,
			Batch:                    100,
			RequestsLimit:            10,
			Peers:                    10,
		},
		fetcher: fetcher,
		db:      db,
		localdb: localdb,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Syncer) requestEpoch(
	ctx context.Context,
	epoch types.EpochID,
	immediate bool,
	consumer chan map[types.ATXID]int,
	complete chan struct{},
) error {
	peers := s.fetcher.SelectBestShuffled(s.cfg.Peers)
	if len(peers) == 0 {
		return fmt.Errorf("no peers available")
	}
	for _, peer := range peers {
		if !immediate {
			select {
			case <-ctx.Done():
				return nil
			case <-complete:
				return nil
			// TODO this part has to be randomized so that different peers don't request at the same time
			case <-time.After(s.cfg.EpochInfoRequestInterval):
			}
		}
		epochData, err := s.fetcher.PeerEpochInfo(ctx, peer, epoch)
		if err != nil || epochData == nil {
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
		s.fetcher.RegisterPeerHashes(peer, types.ATXIDsToHashes(epochData.AtxIDs))
		state := map[types.ATXID]int{}
		for _, atx := range epochData.AtxIDs {
			state[atx] = 0
		}
		select {
		case <-ctx.Done():
			return nil
		case consumer <- state:
		}
	}
	return nil
}

func (s *Syncer) downloadAtxs(
	ctx context.Context,
	epoch types.EpochID,
	state map[types.ATXID]int,
	updates chan map[types.ATXID]int,
) error {
	batch := make([]types.ATXID, 0, s.cfg.Batch)
	batch = batch[:0]
	var (
		lackOfProgress       = 0
		progressTimestamp    time.Time
		downloaded           = map[types.ATXID]bool{}
		previouslyDownloaded = 0
	)

	for {
		// if state is empty we want to block on updates
		// TODO also wait if some activations are unavailable
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

		for atx := range state {
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
			// TODO we should not count timeout failures
			// if requests >= s.cfg.RequestsLimit {
			// 	continue
			// }
			batch = append(batch, atx)
			if len(batch) == cap(batch) {
				break
			}
		}
		// report progress every 10%
		if progress := float64(len(downloaded) - previouslyDownloaded); progress/float64(len(state)) > 0.1 {
			rate := float64(0)
			if previouslyDownloaded != 0 {
				rate = progress / time.Since(progressTimestamp).Seconds()
			}
			previouslyDownloaded = len(downloaded)
			progressTimestamp = time.Now()
			s.logger.Info(
				"atx sync progress",
				epoch.Field().Zap(),
				zap.Int("downloaded", len(downloaded)),
				zap.Int("total", len(state)),
				zap.Float64("rate per sec", rate),
			)
		}
		if len(downloaded) == previouslyDownloaded {
			lackOfProgress++
		}
		if lackOfProgress == 10 {
			lackOfProgress = 0
			s.logger.Warn("lack of progress in atx sync",
				epoch.Field().Zap(),
				zap.Int("downloaded", len(downloaded)),
				zap.Int("total", len(state)),
			)
		}
		if len(batch) == 0 && len(state) != 0 {
			s.logger.Info(
				"atx sync for epoch is finished",
				epoch.Field().Zap(),
				zap.Int("downloaded", len(downloaded)),
				zap.Int("total", len(state)),
				zap.Int("unavailable", len(state)-len(downloaded)),
			)
			return nil
		}
		if err := s.fetcher.GetAtxs(ctx, batch); err != nil {
			s.logger.Debug("failed to download atxs", zap.Error(err))
		}
		for _, atx := range batch {
			// TODO should be updated only if atx failed on timeout
			state[atx]++
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
	s.logger.Info("scheduled atxs downloading", epoch.Field().Zap())

	state, err := atxsync.GetSyncState(s.localdb, epoch)
	if err != nil {
		return fmt.Errorf("failed to get state for epoch %v: %w", epoch, err)
	}
	eg, ctx := errgroup.WithContext(ctx)
	updates := make(chan map[types.ATXID]int, 1)
	complete := make(chan struct{})
	if state == nil {
		state = map[types.ATXID]int{}
	} else {
		updates <- state
	}
	eg.Go(func() error {
		return s.requestEpoch(ctx, epoch, len(state) == 0, updates, complete)
	})
	eg.Go(func() error {
		err := s.downloadAtxs(ctx, epoch, state, updates)
		close(complete)
		return err
	})
	return eg.Wait()
}
