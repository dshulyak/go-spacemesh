package chaos

import (
	"context"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"

	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

// Teardown is returned by every chaos action and executed
// by the caller once chaos needs to be stopped.
type Teardown func(context.Context) error

// Fail the list of pods and prevents them from respawning until teardown is called.
func Fail(cctx *testcontext.Context, name string, pods ...string) (Teardown, error) {
	fail := chaos.PodChaos{}
	fail.Name = name
	fail.Namespace = cctx.Namespace

	fail.Spec.Action = chaos.PodFailureAction
	fail.Spec.Mode = chaos.AllMode
	fail.Spec.Selector = chaos.PodSelectorSpec{
		Pods: map[string][]string{
			cctx.Namespace: pods,
		},
	}
	if err := cctx.Generic.Create(cctx, &fail); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return cctx.Generic.Delete(ctx, &fail)
	}, nil
}

type FailTask struct{}

func (f FailTask) Apply(ctx context.Context, client Client, name string, target Target) (Teardown, error) {
	fail := chaos.PodChaos{}
	fail.Name = name
	fail.Namespace = target.Namespace

	fail.Spec.Action = chaos.PodFailureAction
	fail.Spec.Mode = chaos.AllMode
	fail.Spec.Selector = target.ToSpec()
	if err := client.Create(ctx, &fail); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return client.Delete(ctx, &fail)
	}, nil
}
