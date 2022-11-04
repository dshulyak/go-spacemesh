package chaos

import (
	"context"
	"math/rand"
	"strings"
	"time"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spacemeshos/go-spacemesh/systest/cluster"
)

type Selector func(...*cluster.NodeClient) ([]*cluster.NodeClient, error)

func Chain(selectors ...Selector) Selector {
	return func(clients ...*cluster.NodeClient) ([]*cluster.NodeClient, error) {
		for _, selector := range selectors {
			var err error
			clients, err = selector(clients...)
			if err != nil {
				return nil, err
			}
		}
		return clients, nil
	}
}

func OnlySmeshers(clients ...*cluster.NodeClient) ([]*cluster.NodeClient, error) {
	var rst []*cluster.NodeClient
	for _, client := range clients {
		if strings.HasPrefix(client.Name, "smesher") {
			rst = append(rst, client)
		}
	}
	return rst, nil
}

func NotPoets(clients ...*cluster.NodeClient) ([]*cluster.NodeClient, error) {
	var rst []*cluster.NodeClient
	for _, client := range clients {
		if !strings.HasPrefix(client.Name, "poet") {
			rst = append(rst, client)
		}
	}
	return rst, nil
}

func Fraction(rng *rand.Rand, frac float64) Selector {
	return func(clients ...*cluster.NodeClient) ([]*cluster.NodeClient, error) {
		rng.Shuffle(len(clients), func(i, j int) {
			clients[i], clients[j] = clients[j], clients[i]
		})
		size := int(float64(len(clients)) * frac)
		return clients[:size], nil
	}
}

type Target struct {
	Namespace string
	Pods      []string
}

func (t Target) ToSpec() chaos.PodSelectorSpec {
	spec := chaos.PodSelectorSpec{}
	if len(t.Pods) == 0 {
		spec.Namespaces = []string{t.Namespace}
	} else {
		spec.Pods = map[string][]string{
			t.Namespace: t.Pods,
		}
	}
	return spec
}

type Chaos interface {
	Apply(context.Context, Client, string, Target) (Teardown, error)
}

type Task struct {
	Name     string
	Timed    *Timed
	Selector Selector
	Chaos    Chaos
}

type Timed struct {
	// Duration controls how long Chaos will be applied.
	Duration, Jitter time.Duration
	// Initial time to wait before chaos is applied.
	Initial time.Duration
	// Period is a duration between applying chaos.
	Period time.Duration
}
