package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewCorrectionQueries(db *bun.DB, observe func(auditlog.Observation)) (auditlog.CorrectionQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("audit correction queries: database and observer are required")
	}
	store := postgres.NewCorrections(func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("audit correction queries: tenant is required: %w", err)
		}
		transaction, found := tenant.TransactionFromContext(ctx)
		if !found {
			return db, tenantID.Int64(), nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
		}
		return nil, 0, fmt.Errorf("audit correction queries: unsupported transaction %T", transaction)
	})
	return correctionQueries{store: store, observe: observe}, nil
}

type correctionQueries struct {
	store   *postgres.Corrections
	observe func(auditlog.Observation)
}

func (q correctionQueries) ListDirectCorrections(ctx context.Context, filter auditlog.CorrectionFilter) ([]auditlog.DirectCorrection, error) {
	started := time.Now()
	rows, observation, err := q.store.ListDirectCorrections(ctx, filter)
	observation.Duration, observation.Err = time.Since(started), err
	q.observe(observation)
	return rows, err
}
