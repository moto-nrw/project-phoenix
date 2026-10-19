package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

func NewScopedDatabase[I interface{ Int64() int64 }](current func(context.Context) (any, bool), school func(context.Context) (I, error)) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := current(ctx)
		if !ok {
			return nil, 0, errors.New("export transfer postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("export transfer postgres: unsupported transaction %T", transaction)
		}
		tenantID, err := school(ctx)
		if err != nil {
			return nil, 0, err
		}
		return tx, tenantID.Int64(), nil
	}
}

// uploader binds the transport to the port. Port and transport carry their
// own destination types so neither imports the other; translating between
// them is composition work, and it stays a field copy — any rule about WHERE
// a file may go belongs in the transport's address policy.
