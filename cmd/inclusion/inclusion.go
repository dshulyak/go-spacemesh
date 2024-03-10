package main

import (
	"flag"
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
)

var (
	db   = flag.String("db", "", "database path")
	from = flag.Int("from", 0, "from layer")
	to   = flag.Int("to", 0, "to layer")
)

func main() {
	flag.Parse()
	db, err := sql.Open("file:" + *db)
	must(err)
	var (
		all float64
		n   float64
	)
	for i := *from; i <= *to; i++ {
		included := float64(0)
		id, err := layers.GetApplied(db, types.LayerID(i))
		must(err)
		if id != types.EmptyBlockID {
			block, err := blocks.Get(db, id)
			must(err)
			included += float64(len(block.Rewards))
		}
		ballots, err := ballots.Layer(db, types.LayerID(i))
		must(err)
		total := float64(0)
		for _, ballot := range ballots {
			if ballot.IsMalicious() {
				continue
			}
			total += 1
		}
		all += included / total
		n++
	}
	fmt.Printf("from = %d to = %d average inclusion %f\n", *from, *to, all/n)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
