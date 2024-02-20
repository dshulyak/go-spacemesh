package activeset

import (
	"testing"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/localsql"
	"github.com/stretchr/testify/require"
)

func TestGetNotFound(t *testing.T) {
	const target = 10
	db := localsql.InMemory()
	_, _, _, err := Get(db, Tortoise, target)
	require.ErrorIs(t, err, sql.ErrNotFound)
}

func TestUniquePerEpochPerKind(t *testing.T) {
	const target = 10
	db := localsql.InMemory()
	for _, kind := range []Kind{Tortoise, Hare} {
		require.NoError(t, Add(db, kind, target, [32]byte{1}, 50, nil))
		require.ErrorIs(t, Add(db, kind, target, [32]byte{2}, 50, nil), sql.ErrObjectExists)
	}
}

func TestGet(t *testing.T) {
	const target = 10
	db := localsql.InMemory()

	expectId := [32]byte{1}
	expectWeight := uint64(50)
	expectSet := []types.ATXID{{1}, {2}, {3}}

	require.NoError(t, Add(db, Tortoise, target, expectId, expectWeight, expectSet))
	id, weight, set, err := Get(db, Tortoise, target)
	require.NoError(t, err)
	require.EqualValues(t, expectId, id)
	require.Equal(t, expectWeight, weight)
	require.Equal(t, expectSet, set)
}
