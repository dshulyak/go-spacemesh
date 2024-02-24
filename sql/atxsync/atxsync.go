package atxsync

import (
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
)

func GetSyncState(db sql.Executor, epoch types.EpochID) (map[types.ATXID]int, error) {
	states := map[types.ATXID]int{}
	_, err := db.Exec("select id, requests from atx_sync_state where epoch = ?1",
		func(stmt *sql.Statement) {
			stmt.BindInt64(1, int64(epoch))
		}, func(stmt *sql.Statement) bool {
			var id types.ATXID
			stmt.ColumnBytes(0, id[:])
			states[id] = int(stmt.ColumnInt64(1))
			return true
		})
	if err != nil {
		return nil, fmt.Errorf("select synced atx ids for epoch failed %v: %w", epoch, err)
	}
	return states, nil
}

func SaveSyncState(db sql.Executor, epoch types.EpochID, states map[types.ATXID]int) error {
	for id, requests := range states {
		_, err := db.Exec(`insert into atx_sync_state 
		(epoch, id, requests) values (?1, ?2, ?3)
		on conflict(epoch, id) do update set requests = ?3;`,
			func(stmt *sql.Statement) {
				stmt.BindInt64(1, int64(epoch))
				stmt.BindBytes(2, id[:])
				stmt.BindInt64(3, int64(requests))
			}, nil)
		if err != nil {
			return fmt.Errorf("insert synced atx id %v/%v failed: %w", epoch, id.ShortString(), err)
		}
	}
	return nil
}
