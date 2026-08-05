package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

type EnrollmentRestorationRepository struct {
	db *bun.DB
}

func NewEnrollmentRestorationRepository(db *bun.DB) auditModels.EnrollmentRestorationRepository {
	return &EnrollmentRestorationRepository{db: db}
}

func (r *EnrollmentRestorationRepository) Create(ctx context.Context, event *auditModels.EnrollmentRestoration) error {
	if event == nil {
		return fmt.Errorf("enrollment restoration audit event is required")
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.enrollment_restorations AS "enrollment_restoration"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create enrollment restoration audit event: %w", err)
	}
	return nil
}
