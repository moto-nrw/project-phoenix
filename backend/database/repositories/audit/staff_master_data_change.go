package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// StaffMasterDataChangeRepository is the append-only writer for
// audit.staff_master_data_changes (#1423).
type StaffMasterDataChangeRepository struct {
	runtime Runtime
}

// NewStaffMasterDataChangeRepository creates the audit writer.
func NewStaffMasterDataChangeRepository(runtime Runtime) auditModels.StaffMasterDataChangeCreator {
	return &StaffMasterDataChangeRepository{runtime: requireRuntime(runtime)}
}

// Create inserts one field-level change row.
func (r *StaffMasterDataChangeRepository) Create(ctx context.Context, event *auditModels.StaffMasterDataChange) error {
	if event == nil {
		return fmt.Errorf("staff master data change audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid staff master data change audit event: %w", err)
	}
	return NewAppender(r.runtime).Append(ctx, event)
}
