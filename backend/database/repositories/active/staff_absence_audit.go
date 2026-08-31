package active

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type StaffAbsenceAuditRepository struct {
	db *bun.DB
}

func NewStaffAbsenceAuditRepository(db *bun.DB) active.StaffAbsenceAuditRepository {
	return &StaffAbsenceAuditRepository{db: db}
}

func (r *StaffAbsenceAuditRepository) Create(ctx context.Context, audit *active.StaffAbsenceAudit) error {
	if audit == nil {
		return fmt.Errorf("staff absence audit cannot be nil")
	}
	if err := audit.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, audit)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(audit).
		ModelTableExpr("active.staff_absence_audit").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "create staff absence audit", Err: base.TranslateNotFound(err)}
	}
	return nil
}
