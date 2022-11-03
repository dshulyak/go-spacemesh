package chaos

import (
	"context"
	"fmt"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

// Partition2 partitions pods in array a from pods in array b.
func Partition2(ctx *testcontext.Context, name string, a, b []string) (Teardown, error) {
	partition := chaos.NetworkChaos{}
	partition.Name = name
	partition.Namespace = ctx.Namespace

	partition.Spec.Action = chaos.PartitionAction
	partition.Spec.Mode = chaos.AllMode
	partition.Spec.Selector.Pods = map[string][]string{
		ctx.Namespace: a,
	}
	partition.Spec.Direction = chaos.Both
	partition.Spec.Target = &chaos.PodSelector{
		Mode: chaos.AllMode,
	}
	partition.Spec.Target.Selector.Pods = map[string][]string{
		ctx.Namespace: b,
	}

	desired := partition.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, ctx.Generic, &partition, func() error {
		partition.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("creating partition for %v | %v: %w", a, b, err)
	}

	return func(rctx context.Context) error {
		return ctx.Generic.Delete(rctx, &partition)
	}, nil
}

type Partition2Task struct {
	Left, Right float64
}

func (p Partition2Task) Apply(ctx *testcontext.Context, name string, pods ...string) (Teardown, error) {
	split := int(float64(len(pods)) * p.Left)
	var left, right []string
	for i, pod := range pods {
		if split > 0 && i%2 == 0 {
			left = append(left, pod)
			split--
		} else {
			right = append(right, pod)
		}
	}
	return Partition2(ctx, name, left, right)
}

// Disconnect, unlike Partition, drops all connectivity with the rest of the network.
type Disconnect struct{}

func (Disconnect) Apply(ctx *testcontext.Context, name string, pods ...string) (Teardown, error) {
	disconnect := chaos.NetworkChaos{}
	disconnect.Name = name
	disconnect.Namespace = ctx.Namespace

	disconnect.Spec.Action = chaos.PartitionAction
	disconnect.Spec.Mode = chaos.AllMode
	disconnect.Spec.Selector.Pods = map[string][]string{
		ctx.Namespace: pods,
	}
	disconnect.Spec.Direction = chaos.Both

	disconnect.Spec.Target = &chaos.PodSelector{
		Mode: chaos.AllMode,
	}
	disconnect.Spec.Target.Selector.Namespaces = []string{ctx.Namespace}

	desired := disconnect.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, ctx.Generic, &disconnect, func() error {
		disconnect.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("disconnecting %v: %w", pods, err)
	}

	return func(rctx context.Context) error {
		return ctx.Generic.Delete(rctx, &disconnect)
	}, nil
}
