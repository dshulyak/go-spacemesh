package transactions

import (
	"fmt"

	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/sql"
)

// Add transaction to the database. If transaction already exists layer and block will be updated.
func Add(db sql.Executor, lid types.LayerID, bid types.BlockID, tx *types.Transaction) error {
	buf, err := codec.Encode(tx)
	if err != nil {
		return fmt.Errorf("encode %+v: %w", tx, err)
	}
	if _, err := db.Exec(`insert into transactions 
	(id, tx, layer, block, origin, destination) 
	values (?1, ?2, ?3, ?4, ?5, ?6) 
	on conflict(id) do
	update set layer = ?3, block=?4`,
		func(stmt *sql.Statement) {
			stmt.BindBytes(1, tx.ID().Bytes())
			stmt.BindBytes(2, buf)
			stmt.BindInt64(3, int64(lid.Value))
			stmt.BindBytes(4, bid.Bytes())
			stmt.BindBytes(5, tx.Origin().Bytes())
			stmt.BindBytes(6, tx.Recipient.Bytes())
		}, nil); err != nil {
		return fmt.Errorf("insert %s: %w", tx.ID(), err)
	}
	return nil
}

// Applied update transaction when it is no longer pending.
func Applied(db sql.Executor, id types.TransactionID) error {
	if rows, err := db.Exec("update transactions set applied = true where id = ?1 returning id", func(stmt *sql.Statement) {
		stmt.BindBytes(1, id.Bytes())
	}, nil); err != nil {
		return fmt.Errorf("applied %s: %w", id, err)
	} else if rows == 0 {
		return fmt.Errorf("%w: tx %s", sql.ErrNotFound, id)
	}
	return nil
}

// Delete transaction from database.
func Delete(db sql.Executor, id types.TransactionID) error {
	if _, err := db.Exec("delete from transactions where id = ?1",
		func(stmt *sql.Statement) {
			stmt.BindBytes(1, id.Bytes())
		}, nil); err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	return nil
}

// tx, layer, block, origin.
func decodeTransaction(id types.TransactionID, stmt *sql.Statement) (*types.MeshTransaction, error) {
	var (
		tx     types.Transaction
		origin types.Address
		bid    types.BlockID
	)
	if _, err := codec.DecodeFrom(stmt.ColumnReader(0), &tx); err != nil {
		return nil, fmt.Errorf("decode %w", err)
	}
	lid := types.NewLayerID(uint32(stmt.ColumnInt64(1)))
	stmt.ColumnBytes(2, bid[:])
	stmt.ColumnBytes(3, origin[:])
	tx.SetOrigin(origin)
	tx.SetID(id)
	return &types.MeshTransaction{
		Transaction: tx,
		LayerID:     lid,
		BlockID:     bid,
	}, nil
}

// Get transaction from database.
func Get(db sql.Executor, id types.TransactionID) (tx *types.MeshTransaction, err error) {
	if rows, err := db.Exec("select tx, layer, block, origin from transactions where id = ?1",
		func(stmt *sql.Statement) {
			stmt.BindBytes(1, id.Bytes())
		}, func(stmt *sql.Statement) bool {
			tx, err = decodeTransaction(id, stmt)
			return true
		}); err != nil {
		return nil, fmt.Errorf("get %s: %w", id, err)
	} else if rows == 0 {
		return nil, fmt.Errorf("%w: tx %s", sql.ErrNotFound, id)
	}
	return tx, err
}

// Has returns true if transaction is stored in the database.
func Has(db sql.Executor, id types.TransactionID) (bool, error) {
	rows, err := db.Exec("select 1 from transactions where id = ?1",
		func(stmt *sql.Statement) {
			stmt.BindBytes(1, id.Bytes())
		}, nil)
	if err != nil {
		return false, fmt.Errorf("has %s: %w", id, err)
	}
	return rows > 0, nil
}

// query MUST ensure that this order of fields tx, layer, block, origin, id.
func filter(db sql.Executor, query string, encoder func(*sql.Statement)) (rst []*types.MeshTransaction, err error) {
	if _, err := db.Exec(query, encoder, func(stmt *sql.Statement) bool {
		var (
			tx *types.MeshTransaction
			id types.TransactionID
		)
		stmt.ColumnBytes(4, id[:])
		tx, err = decodeTransaction(id, stmt)
		if err != nil {
			return false
		}
		rst = append(rst, tx)
		return true
	}); err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	return rst, err
}

func filterByAddress(db sql.Executor, addrfield string, from, to types.LayerID, address types.Address) ([]*types.MeshTransaction, error) {
	return filter(db, fmt.Sprintf(`select tx, layer, block, origin, id from transactions
		where %s = ?1 and layer >= ?2 and layer <= ?3`, addrfield), func(stmt *sql.Statement) {
		stmt.BindBytes(1, address[:])
		stmt.BindInt64(2, int64(from.Value))
		stmt.BindInt64(3, int64(to.Value))
	})
}

const (
	originField      = "origin"
	destinationField = "destination"
)

// FilterByOrigin filter transaction by origin [from, to] layers.
func FilterByOrigin(db sql.Executor, from, to types.LayerID, address types.Address) ([]*types.MeshTransaction, error) {
	return filterByAddress(db, originField, from, to, address)
}

// FilterByDestination filter transaction by destnation [from, to] layers.
func FilterByDestination(db sql.Executor, from, to types.LayerID, address types.Address) ([]*types.MeshTransaction, error) {
	return filterByAddress(db, destinationField, from, to, address)
}

// FilterPending filters all transactions that are not yet applied.
func FilterPending(db sql.Executor, address types.Address) ([]*types.MeshTransaction, error) {
	return filter(db, `select tx, layer, block, origin, id from transactions
		where origin = ?1 and applied = false`, func(stmt *sql.Statement) {
		stmt.BindBytes(1, address[:])
	})
}
