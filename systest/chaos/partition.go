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

func (p Partition2Task) Apply(ctx context.Context, client Client, name string, target Target) (Teardown, error) {
	pods := target.Pods
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
	spec := chaos.NetworkChaos{}
	spec.Name = name
	spec.Namespace = target.Namespace

	spec.Spec.Action = chaos.PartitionAction
	spec.Spec.Mode = chaos.AllMode
	spec.Spec.Selector.Pods = map[string][]string{target.Namespace: left}
	spec.Spec.Direction = chaos.Both

	spec.Spec.Target = &chaos.PodSelector{
		Mode: chaos.AllMode,
	}
	spec.Spec.Target.Selector.Pods = map[string][]string{target.Namespace: right}
	desired := spec.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, client, &spec, func() error {
		spec.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return client.Delete(ctx, &spec)
	}, nil
}

// Disconnect, unlike Partition, drops all connectivity with the rest of the network.
type Disconnect struct{}

func (Disconnect) Apply(ctx context.Context, client Client, name string, target Target) (Teardown, error) {
	spec := chaos.NetworkChaos{}
	spec.Name = name
	spec.Namespace = target.Namespace

	spec.Spec.Action = chaos.PartitionAction
	spec.Spec.Mode = chaos.AllMode
	spec.Spec.Selector = target.ToSpec()
	spec.Spec.Direction = chaos.Both

	spec.Spec.Target = &chaos.PodSelector{
		Mode: chaos.AllMode,
	}
	spec.Spec.Target.Selector.Namespaces = []string{target.Namespace}
	desired := spec.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, client, &spec, func() error {
		spec.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return client.Delete(ctx, &spec)
	}, nil
}
