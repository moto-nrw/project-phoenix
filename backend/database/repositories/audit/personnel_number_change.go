package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

type PersonnelNumberChangeRepository struct {
	db *bun.DB
}

func NewPersonnelNumberChangeRepository(db *bun.DB) auditModels.PersonnelNumberChangeRepository {
	return &PersonnelNumberChangeRepository{db: db}
}

func (r *PersonnelNumberChangeRepository) Create(ctx context.Context, event *auditModels.PersonnelNumberChange) error {
	if event == nil {
		return fmt.Errorf("personnel number change audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid personnel number change audit event: %w", err)
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.personnel_number_changes AS "personnel_number_change"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create personnel number change audit event: %w", err)
	}
	return nil
}
