package atxsync

import (
	"context"
	"testing"

	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/localsql"
	"github.com/stretchr/testify/require"
)

func TestSyncer(t *testing.T) {
	localdb := localsql.InMemory()
	db := sql.InMemory()
	syncer := New(nil, nil, db, localdb)
	require.NoError(t, syncer.Download(context.Background(), 1))
}
