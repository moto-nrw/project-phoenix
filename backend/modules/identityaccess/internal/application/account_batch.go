package application

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
)

type AccountBatchStore interface {
	FindActiveAccounts(context.Context, []int64) (map[int64]domain.Account, domain.OperationStats, error)
}

// NewAccountBatch reads current account authority in the caller's unit,
// without widening the authentication/command facade or retaining a cache.
func NewAccountBatch(store AccountBatchStore, observe ports.Observer) identityaccess.AccountBatchQuery {
	if store == nil || observe == nil {
		panic("identity access: account batch requires all dependencies")
	}
	return func(ctx context.Context, ids []int64) (map[int64]identityaccess.Account, error) {
		result := make(map[int64]identityaccess.Account)
		started := time.Now()
		rows, measured, err := store.FindActiveAccounts(ctx, ids)
		if err == nil {
			for id, row := range rows {
				result[id] = identityaccess.Account(row)
			}
		}
		observe(ports.Observation{Operation: "find_active_accounts", Duration: time.Since(started), Stats: measured, Err: err})
		return result, MapError(err)
	}
}
