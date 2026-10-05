package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type timetableBlockDisplayMetadata struct{ db *bun.DB }

func newTimetableBlockDisplayMetadata(db *bun.DB) timetableBlockDisplayMetadata {
	if db == nil {
		panic("timetable block display metadata: database is required")
	}
	return timetableBlockDisplayMetadata{db: db}
}

func (r timetableBlockDisplayMetadata) ListBlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := timetableBlockDisplayDatabase(ctx, r.db)
	if err != nil {
		return nil, err
	}
	return timetableblockdisplay.List(ctx, db, tenantID.Int64(), instanceIDs)
}

func timetableBlockDisplayDatabase(ctx context.Context, db *bun.DB) (bun.IDB, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return db, nil
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return tx, nil
		}
		return db, nil
	default:
		return nil, fmt.Errorf("timetable block display metadata: unsupported transaction %T", raw)
	}
}
