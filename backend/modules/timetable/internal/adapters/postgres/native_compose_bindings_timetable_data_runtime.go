package postgres

import (
	context "context"
	fmt "fmt"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

type BlockDisplayMetadata struct {
	Db      *bun.DB
	Project BlockDisplayProjection
}

func (r BlockDisplayMetadata) ListBlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := BlockDisplayDatabase(ctx, r.Db)
	if err != nil {
		return nil, err
	}
	return r.Project(ctx, db, tenantID.Int64(), instanceIDs)
}

func BlockDisplayDatabase(ctx context.Context, db *bun.DB) (bun.IDB, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("timetable block display metadata: transaction is required")
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
		return nil, fmt.Errorf("timetable block display metadata: transaction is required")
	default:
		return nil, fmt.Errorf("timetable block display metadata: unsupported transaction %T", raw)
	}
}

type BlockDisplayProjection func(ctx context.Context, db bun.IDB, tenantID int64, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error)
