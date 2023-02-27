package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/p2p/bootstrap"
	"github.com/spacemeshos/go-spacemesh/p2p/handshake"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
)

// Opt is for configuring Host.
type Opt func(fh *Host)

// WithLog configures logger for Host.
func WithLog(logger log.Log) Opt {
	return func(fh *Host) {
		fh.logger = logger
	}
}

// WithConfig sets Config for Host.
func WithConfig(cfg Config) Opt {
	return func(fh *Host) {
		fh.cfg = cfg
	}
}

// WithContext set context for Host.
func WithContext(ctx context.Context) Opt {
	return func(fh *Host) {
		fh.ctx = ctx
	}
}

// WithNodeReporter updates reporter that is notified every time when
// node added or removed a peer.
func WithNodeReporter(reporter func()) Opt {
	return func(fh *Host) {
		fh.nodeReporter = reporter
	}
}

// Host is a conveniency wrapper for all p2p related functionality required to run
// a full spacemesh node.
type Host struct {
	ctx    context.Context
	cfg    Config
	logger log.Log

	host.Host
	*pubsub.PubSub

	nodeReporter func()
	*bootstrap.Peers

	hs        *handshake.Handshake
	bootstrap *bootstrap.Bootstrap
}

// Upgrade creates Host instance from host.Host.
func Upgrade(h host.Host, genesisID types.Hash20, opts ...Opt) (*Host, error) {
	fh := &Host{
		ctx:    context.Background(),
		cfg:    DefaultConfig(),
		logger: log.NewNop(),
		Host:   h,
	}
	for _, opt := range opts {
		opt(fh)
	}
	router, err := pubsub.New(fh.ctx, fh.logger, h, pubsub.Config{
		Flood:          fh.cfg.Flood,
		IsBootnode:     fh.cfg.IsBootnode,
		MaxMessageSize: fh.cfg.MaxMessageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pubsub: %w", err)
	}
	if fh.bootstrap, err = bootstrap.NewBootstrap(fh.logger, fh); err != nil {
		return nil, fmt.Errorf("failed to initiliaze bootstrap: %w", err)
	}
	fh.PubSub = router
	fh.Peers = bootstrap.StartPeers(h,
		bootstrap.WithLog(fh.logger),
		bootstrap.WithContext(fh.ctx),
		bootstrap.WithNodeReporter(fh.nodeReporter),
	)
	fh.hs = handshake.New(fh, genesisID, handshake.WithLog(fh.logger))
	return fh, nil
}

// Stop background workers and release external resources.
func (fh *Host) Stop() error {
	fh.bootstrap.Stop()
	fh.Peers.Stop()
	fh.hs.Stop()
	if err := fh.Host.Close(); err != nil {
		return fmt.Errorf("failed to close libp2p host: %w", err)
	}
	return nil
}
