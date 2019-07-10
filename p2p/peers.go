package p2p

import (
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/p2p/p2pcrypto"
	"sync/atomic"
)

type Peer p2pcrypto.PublicKey

type Peers interface {
	GetPeers() []Peer
	Close()
}

type PeersImpl struct {
	log.Log
	snapshot *atomic.Value
	exit     chan struct{}
}

// NewPeersImpl creates a PeersImpl using specified parameters and returns it
func NewPeersImpl(snapshot *atomic.Value, exit chan struct{}, lg log.Log) *PeersImpl {
	return &PeersImpl{snapshot: snapshot, Log: lg, exit: exit}
}

type peerProvider interface {
	SubscribePeerEvents() (chan p2pcrypto.PublicKey, chan p2pcrypto.PublicKey)
}

func NewPeers(s peerProvider, lg log.Log) Peers {
	value := atomic.Value{}
	value.Store(make([]Peer, 0, 20))
	pi := NewPeersImpl(&value, make(chan struct{}), lg)
	newPeerC, expiredPeerC := s.SubscribePeerEvents()
	go pi.listenToPeers(newPeerC, expiredPeerC)
	return pi
}

func (pi PeersImpl) Close() {
	close(pi.exit)
}

func (pi PeersImpl) GetPeers() []Peer {
	peers := pi.snapshot.Load().([]Peer)
	pi.Info("now connected to %v peers", len(peers))
	return peers
}

func (pi *PeersImpl) listenToPeers(newPeerC chan p2pcrypto.PublicKey, expiredPeerC chan p2pcrypto.PublicKey) {
	peerSet := make(map[Peer]bool) //set of uniq peers
	for {
		select {
		case <-pi.exit:
			pi.Debug("run stopped")
			return
		case peer := <-newPeerC:
			pi.Debug("new peer %v", peer.String())
			peerSet[peer] = true
		case peer := <-expiredPeerC:
			pi.Debug("expired peer %v", peer.String())
			delete(peerSet, peer)
		}
		keys := make([]Peer, 0, len(peerSet))
		for k := range peerSet {
			keys = append(keys, k)
		}
		pi.snapshot.Store(keys) //swap snapshot
	}
}
