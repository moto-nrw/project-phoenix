package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// StaffMasterDataChangeRepository is the append-only writer for
// audit.staff_master_data_changes (#1423).
type StaffMasterDataChangeRepository struct {
	db *bun.DB
}

// NewStaffMasterDataChangeRepository creates the audit writer.
func NewStaffMasterDataChangeRepository(db *bun.DB) auditModels.StaffMasterDataChangeCreator {
	return &StaffMasterDataChangeRepository{db: db}
}

// Create inserts one field-level change row.
func (r *StaffMasterDataChangeRepository) Create(ctx context.Context, event *auditModels.StaffMasterDataChange) error {
	if event == nil {
		return fmt.Errorf("staff master data change audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid staff master data change audit event: %w", err)
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.staff_master_data_changes AS "staff_master_data_change"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create staff master data change audit event: %w", err)
	}
	return nil
}
