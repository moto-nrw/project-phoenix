package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// GuardianFinancialChangeRepository is the append-only writer for
// audit.guardian_financial_changes (#2608).
type GuardianFinancialChangeRepository struct {
	db *bun.DB
}

// NewGuardianFinancialChangeRepository creates the audit writer.
func NewGuardianFinancialChangeRepository(db *bun.DB) auditModels.GuardianFinancialChangeCreator {
	return &GuardianFinancialChangeRepository{db: db}
}

// Create inserts one field-level change row.
func (r *GuardianFinancialChangeRepository) Create(ctx context.Context, event *auditModels.GuardianFinancialChange) error {
	if event == nil {
		return fmt.Errorf("guardian financial change audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid guardian financial change audit event: %w", err)
	}
	base.EnsureTenantID(ctx, event)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(`audit.guardian_financial_changes AS "guardian_financial_change"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create guardian financial change audit event: %w", err)
	}
	return nil
}
