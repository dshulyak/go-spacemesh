package chaos

import (
	"context"
	"fmt"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

type Delay struct {
	Latency string
}

func (d Delay) Apply(ctx *testcontext.Context, name string, pods ...string) (Teardown, error) {
	net := chaos.NetworkChaos{}
	net.Name = name
	net.Namespace = ctx.Namespace

	net.Spec.Action = chaos.DelayAction
	net.Spec.Mode = chaos.AllMode
	net.Spec.Selector.Pods = map[string][]string{
		ctx.Namespace: pods,
	}
	net.Spec.Direction = chaos.Both
	net.Spec.Target = &chaos.PodSelector{
		Mode: chaos.AllMode,
	}
	net.Spec.Target.Selector.Namespaces = []string{ctx.Namespace}

	net.Spec.Delay = &chaos.DelaySpec{
		Latency: d.Latency,
	}
	desired := net.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, ctx.Generic, &net, func() error {
		net.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bandwidth %v: %w", pods, err)
	}
	return func(rctx context.Context) error {
		return ctx.Generic.Delete(rctx, &net)
	}, nil
}
