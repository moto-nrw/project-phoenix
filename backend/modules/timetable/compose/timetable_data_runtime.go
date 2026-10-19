package compose

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BlockDisplayProjection lists a tenant's block display metadata over the
// handle the caller's tenant transaction holds. The legacy composition binds
// the block display projection, which this composition may not name.
type BlockDisplayProjection func(ctx context.Context, db bun.IDB, tenantID int64, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error)

type blockDisplayMetadata struct {
	db      *bun.DB
	project BlockDisplayProjection
}

// NewBlockDisplayMetadata serves the planner's BlockMetadata port from the
// block display projection over db, inside the ambient tenant transaction
// supplied by the application workflow.
func NewBlockDisplayMetadata(db *bun.DB, project BlockDisplayProjection) ports.BlockDisplayMetadata {
	if db == nil {
		panic("timetable block display metadata: database is required")
	}
	if project == nil {
		panic("timetable block display metadata: projection is required")
	}
	return blockDisplayMetadata{db: db, project: project}
}

func (r blockDisplayMetadata) ListBlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	db, err := blockDisplayDatabase(ctx, r.db)
	if err != nil {
		return nil, err
	}
	return r.project(ctx, db, tenantID.Int64(), instanceIDs)
}

func blockDisplayDatabase(ctx context.Context, db *bun.DB) (bun.IDB, error) {
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
