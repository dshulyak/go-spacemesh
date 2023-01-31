package transactions

import (
	"context"
	"errors"
	"time"

	pb "github.com/spacemeshos/api/release/go/spacemesh/v1"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/systest/cluster"
)

type walletActor interface {
	Address() string
	Next(context.Context) ([][]byte, error)
	OnTx(*Tx)
	OnReward(*Reward)
	OnLayer(uint32, string)
}

func collectRewards(ctx context.Context, node *cluster.NodeClient, wallet walletActor) error {
	client := pb.NewGlobalStateServiceClient(node)
	filter := pb.GlobalStateDataFlag_GLOBAL_STATE_DATA_FLAG_REWARD | pb.GlobalStateDataFlag_GLOBAL_STATE_DATA_FLAG_GLOBAL_STATE_HASH
	stream, err := client.GlobalStateStream(ctx, &pb.GlobalStateStreamRequest{
		GlobalStateDataFlags: uint32(filter),
	})
	if err != nil {
		return err
	}
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		if d := resp.GetDatum(); d != nil {
			if r := d.GetReward(); r != nil {
				wallet.OnReward(&Reward{
					Layer:    r.Layer.Number,
					Coinbase: r.Coinbase.Address,
					Amount:   r.Total.Value,
				})
			} else if s := d.GetGlobalState(); s != nil {
				wallet.OnLayer(s.Layer.Number, string(s.RootHash))
			}
		}
	}
}

func collectTxResults(ctx context.Context, node *cluster.NodeClient, wallet walletActor) error {
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
}

func Run(ctx context.Context, node *cluster.NodeClient, wallet walletActor) error {
	// on the start read all transactions and rewards
	for {
		var eg errgroup.Group
		eg.Go(func() error {
			return collectRewards(ctx, node, wallet)
		})
		eg.Go(func() error {
			return collectTxResults(ctx, node, wallet)
		})
		eg.Go(func() error {
			// submit transactions
			txapi := pb.NewTransactionServiceClient(node)
			validate := every(2*time.Minute, func(ctx context.Context) error {
				return nil
			})
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
				if err := validate(ctx); err != nil {
					return err
				}
			}
		})
		if err := eg.Wait(); errors.Is(err, context.Canceled) {
			return nil
		}
	}
}

type fn func(context.Context) error

func every(period time.Duration, f fn) fn {
	last := time.Time{}
	return func(ctx context.Context) error {
		if time.Since(last) > period {
			last = time.Now()
			return f(ctx)
		}
		return nil
	}
}
