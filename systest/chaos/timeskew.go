package chaos

import (
	"context"
	"time"

	chaos "github.com/chaos-mesh/chaos-mesh/api/v1alpha1"

	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

// Timeskew adjusts CLOCK_REALTIME on the specified pods by the offset.
func Timeskew(cctx *testcontext.Context, name string, offset string, pods ...string) (Teardown, error) {
	tc := chaos.TimeChaos{}
	tc.Name = name
	tc.Namespace = cctx.Namespace

	tc.Spec.Mode = chaos.AllMode
	tc.Spec.Selector = chaos.PodSelectorSpec{
		Pods: map[string][]string{
			cctx.Namespace: pods,
		},
	}
	tc.Spec.TimeOffset = offset

	if err := cctx.Generic.Create(cctx, &tc); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return cctx.Generic.Delete(ctx, &tc)
	}, nil
}

// TimeskewTask adjust CLOCK_REALTIME by the specified offset (offset can be positive or negative).
type TimeskewTask struct {
	Offset time.Duration
}

func (t TimeskewTask) Apply(ctx context.Context, client Client, name string, target Target) (Teardown, error) {
	tc := chaos.TimeChaos{}
	tc.Name = name
	tc.Namespace = target.Namespace

	tc.Spec.Mode = chaos.AllMode
	tc.Spec.Selector = target.ToSpec()
	tc.Spec.TimeOffset = t.Offset.String()

	if err := client.Create(ctx, &tc); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return client.Delete(ctx, &tc)
	}, nil
}
