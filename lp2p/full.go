package lp2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"

	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/lp2p/bootstrap"
	"github.com/spacemeshos/go-spacemesh/lp2p/compat"
	"github.com/spacemeshos/go-spacemesh/lp2p/handshake"
	"github.com/spacemeshos/go-spacemesh/lp2p/peerexchange"
	"github.com/spacemeshos/go-spacemesh/lp2p/pubsub"
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

// Host is a conveniency wrapper for all p2p related functionality required to run
// a full spacemesh node.
type Host struct {
	ctx    context.Context
	cfg    Config
	logger log.Log

	host.Host
	*pubsub.PubSub
	*bootstrap.Peers
	discovery *peerexchange.Discovery
	hs        *handshake.Handshake
	bootstrap *bootstrap.Bootstrap
}

// Wrap creates Host instance from host.Host.
func Wrap(h host.Host, opts ...Opt) (*Host, error) {
	fh := &Host{
		ctx:    context.Background(),
		cfg:    Default(),
		logger: log.NewNop(),
		Host:   h,
	}
	for _, opt := range opts {
		opt(fh)
	}
	cfg := fh.cfg
	var err error
	fh.PubSub, err = pubsub.New(fh.ctx, fh.logger, h, pubsub.Config{
		Flood:          cfg.Flood,
		MaxMessageSize: cfg.MaxMessageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pubsub: %w", err)
	}
	fh.Peers = bootstrap.StartPeers(h, bootstrap.WithLog(fh.logger))
	fh.discovery, err = peerexchange.New(fh.logger, h, peerexchange.Config{
		DataDir:   cfg.DataDir,
		Bootnodes: cfg.Bootnodes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize peerexchange discovery: %w", err)
	}
	fh.bootstrap, err = bootstrap.NewBootstrap(fh.logger, bootstrap.Config{
		TargetOutbound: cfg.TargetOutbound,
		Timeout:        cfg.BootstrapTimeout,
	}, fh, fh.discovery)
	if err != nil {
		return nil, fmt.Errorf("failed to initiliaze bootstrap: %w", err)
	}
	fh.hs = handshake.New(fh, cfg.NetworkID, handshake.WithLog(fh.logger))
	return fh, nil
}

// Stop background workers and release external resources.
func (fh *Host) Stop() error {
	fh.discovery.Stop()
	fh.bootstrap.Stop()
	fh.Peers.Stop()
	fh.hs.Stop()
	compat.CloseWriter()
	if err := fh.Host.Close(); err != nil {
		return fmt.Errorf("failed to close libp2p host: %w", err)
	}
	return nil
}
