package transactions

import (
	"context"
	"errors"
	"fmt"

	pb "github.com/spacemeshos/api/release/go/spacemesh/v1"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/systest/cluster"
)

type walletActor interface {
	Address() string
	Next(context.Context) ([][]byte, error)
	OnTx(*Tx)
	OnReward(*Reward)
}

func run(ctx context.Context, node *cluster.NodeClient, wallet walletActor) error {
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
				wallet.OnReward(&Reward{
					Layer:    r.Layer.Number,
					Coinbase: r.Coinbase.Address,
					Amount:   r.Total.Value,
				})
			}
		})
		eg.Go(func() error {
			client := pb.NewTransactionServiceClient(node)
			stream, err := client.StreamResults(ctx, &pb.TransactionResultsRequest{
				Address: wallet.Address(),
				Watch:   true,
			})
			if err != nil {
				return err
			}
			for {
				rst, err := stream.Recv()
				if err != nil {
					return err
				}
				wallet.OnTx(&Tx{
					Layer:     rst.Layer,
					Principal: rst.Tx.Principal.Address,
					Updated:   rst.TouchedAddresses,
					Template:  rst.Tx.Template.Address,
					Method:    uint8(rst.Tx.Method),
					Failed:    rst.Status == pb.TransactionResult_FAILURE,
					Fee:       rst.Fee,
					Nonce:     rst.Tx.Nonce.Counter,
					Raw:       rst.Tx.Raw,
				})
			}
		})
		eg.Go(func() error {
			// submit transactions
			txapi := pb.NewTransactionServiceClient(node)
			for {
				txs, err := wallet.Next(ctx)
				if err != nil {
					return err
				}
				for _, tx := range txs {
					_, err := txapi.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{
						Transaction: tx,
					})
					if err != nil {
						return err
					}
				}
			}
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
