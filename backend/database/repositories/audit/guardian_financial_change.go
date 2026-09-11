package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// GuardianFinancialChangeRepository is the append-only writer for
// audit.guardian_financial_changes (#2608).
type GuardianFinancialChangeRepository struct {
	runtime Runtime
}

// NewGuardianFinancialChangeRepository creates the audit writer.
func NewGuardianFinancialChangeRepository(runtime Runtime) auditModels.GuardianFinancialChangeCreator {
	return &GuardianFinancialChangeRepository{runtime: requireRuntime(runtime)}
}

// Create inserts one field-level change row.
func (r *GuardianFinancialChangeRepository) Create(ctx context.Context, event *auditModels.GuardianFinancialChange) error {
	if event == nil {
		return fmt.Errorf("guardian financial change audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid guardian financial change audit event: %w", err)
	}
	return NewAppender(r.runtime).Append(ctx, event)
}
