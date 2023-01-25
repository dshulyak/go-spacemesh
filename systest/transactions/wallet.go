package transactions

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/spacemeshos/api/release/go/spacemesh/v1"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/systest/cluster"
)

func run(ctx context.Context, node *cluster.NodeClient, address string) error {
	var s state
	// on the start read all transactions and rewards

	for {
		var eg errgroup.Group
		eg.Go(func() error {
			client := pb.NewGlobalStateServiceClient(node)
			stream, err := client.GlobalStateStream(ctx, &pb.GlobalStateStreamRequest{
				GlobalStateDataFlags: uint32(pb.GlobalStateDataFlag_GLOBAL_STATE_DATA_FLAG_REWARD),
			})
			if err != nil {
				return err
			}
			for {
				resp, err := stream.Recv()
				if err != nil {
					return err
				}
				r := resp.GetDatum().GetReward()
				if r == nil {
					return fmt.Errorf("got something else instead of reward")
				}
				s.OnReward(&reward{
					layer:    r.Layer.Number,
					coinbase: r.Coinbase.Address,
					amount:   r.Total.Value,
				})
			}
		})
		eg.Go(func() error {
			client := pb.NewTransactionServiceClient(node)
			stream, err := client.StreamResults(ctx, &pb.TransactionResultsRequest{
				Address: address,
				Watch:   true,
			})
			if err != nil {
				return err
			}
			for {
				resp, err := stream.Recv()
				if err != nil {
					return err
				}
				s.OnTx(toTx(resp))
			}
		})
		eg.Go(func() error {
			// submit transactions
			if s.NeedsSpawn() {
				// spawn
			}
			if !s.WaitSpawned(ctx) {
				return ctx.Err()
			}
			// customized workload
			// 1. spawn other contracts
			// 2. spend funds
			// 3. drain vault
			return nil
		})
		eg.Go(func() error {
			// track revert and update state accordingly
			return nil
		})
		eg.Go(func() error {
			// periodic assertion
			// is it generic or custom per wallet kind?
			return nil
		})
		if err := eg.Wait(); errors.Is(err, context.Canceled) {
			return nil
		}
	}
}

func toTx(rst *pb.TransactionResult) *tx {
	return &tx{
		layer:     rst.Layer,
		principal: rst.Tx.Principal.Address,
		updated:   rst.TouchedAddresses,
		template:  rst.Tx.Template.Address,
		method:    uint8(rst.Tx.Method),
		failed:    rst.Status == pb.TransactionResult_FAILURE,
		fee:       rst.Fee,
		nonce:     rst.Tx.Nonce.Counter,
		raw:       rst.Tx.Raw,
	}
}
