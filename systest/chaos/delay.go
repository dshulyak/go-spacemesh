package chaos

import (
	"context"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Delay struct {
	Latency string
}

func (d Delay) Apply(ctx context.Context, client Client, name string, target Target) (Teardown, error) {
	spec := chaos.NetworkChaos{}
	spec.Name = name
	spec.Namespace = target.Namespace

	spec.Spec.Action = chaos.DelayAction
	spec.Spec.Mode = chaos.AllMode
	spec.Spec.Selector = target.ToSpec()
	spec.Spec.Direction = chaos.Both
	spec.Spec.Target = &chaos.PodSelector{
		Mode: chaos.AllMode,
	}
	spec.Spec.Target.Selector.Namespaces = []string{target.Namespace}

	spec.Spec.Delay = &chaos.DelaySpec{
		Latency: d.Latency,
	}
	desired := spec.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, client, &spec, func() error {
		spec.Spec = desired.Spec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return func(rctx context.Context) error {
		return client.Delete(rctx, &spec)
	}, nil
}
