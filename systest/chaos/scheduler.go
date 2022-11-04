package chaos

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spacemeshos/go-spacemesh/systest/cluster"
	"github.com/spacemeshos/go-spacemesh/systest/testcontext"
)

type Client = client.Client

func NewScheduler(tctx *testcontext.Context, rng *rand.Rand, cl *cluster.Cluster, tasks []Task) *Scheduler {
	s := &Scheduler{
		rng: rng, cluster: cl,
		namespace: tctx.Namespace, client: tctx.Generic,
	}
	for _, task := range tasks {
		stask := &schedulableTask{Task: task}
		if task.Timed != nil {
			end := time.Now().Add(task.Timed.Initial)
			stask.end = &end
		}
		s.tasks = append(s.tasks, stask)
	}
	return s
}

type Scheduler struct {
	rng       *rand.Rand
	log       *zap.SugaredLogger
	cluster   *cluster.Cluster
	client    Client
	namespace string

	tasks []*schedulableTask
}

type schedulableTask struct {
	start, end *time.Time
	duration   time.Duration
	teardown   Teardown
	Task
}

// Next schedules tasks on the pods according to the timing contstrains.
// Not safe to call concurrently.
func (s *Scheduler) Next(ctx context.Context) error {
	var clients []*cluster.NodeClient
	for i := 0; i < s.cluster.Total(); i++ {
		clients = append(clients, s.cluster.Client(i))
	}
	for i := 0; i < s.cluster.Poets(); i++ {
		clients = append(clients, s.cluster.Poet(i))
	}
	eg := errgroup.Group{}
	for _, task := range s.tasks {
		task := task
		eg.Go(func() error {
			if task.start != nil {
				if time.Since(*task.start) < task.duration {
					return nil
				}

				now := time.Now()
				task.end = &now
				teardown := task.teardown

				task.start = nil
				task.duration = 0
				task.teardown = nil

				s.log.Infow("stoping chaos task", "name", task.Name)
				return teardown(ctx)
			}
			if task.end != nil && task.Timed != nil {
				if time.Since(*task.end) < task.Timed.Period {
					return nil
				}
				task.end = nil
			}
			if task.Timed != nil {
				task.duration = task.Timed.Duration + time.Duration(s.rng.Int63n(int64(task.Timed.Jitter.Nanoseconds())))
			}
			target := Target{Namespace: s.namespace}
			if task.Selector != nil {
				var err error
				clients, err = task.Selector(clients...)
				if err != nil {
					return err
				}
				target.Pods = []string{}
				target.Pods = append(target.Pods, extractNames(clients)...)
			}

			s.log.Infow("applying chaos task", "name", task.Name, "namespace", target.Namespace, "pods", target.Pods)
			teardown, err := task.Chaos.Apply(ctx, s.client, task.Name, target)
			if err != nil {
				return err
			}
			now := time.Now()
			if task.Timed != nil {
				task.start = &now
			}
			task.teardown = teardown
			return nil
		})

	}
	return eg.Wait()
}

func (s *Scheduler) Run(ctx context.Context, tick time.Duration) error {
	var (
		timer = time.NewTimer(tick)
		err   error
	)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			err = s.Next(ctx)
			timer.Reset(tick)
		case <-ctx.Done():
			err = ctx.Err()
		}
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				s.log.Errorf("scheduler is shutting down on failure", "error", err)
			}
			for _, task := range s.tasks {
				if task.teardown != nil {
					if err := task.teardown(ctx); err != nil {
						s.log.Errorw("failed to teardown task on shutdown",
							"task", task.Name, "error", err,
						)
					}
				}
			}
		}
	}
}

func extractNames(clients []*cluster.NodeClient) (rst []string) {
	for _, client := range clients {
		rst = append(rst, client.Name)
	}
	return rst
}
