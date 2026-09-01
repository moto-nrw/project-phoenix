package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// ClassListEntryChangeRepository is the append-only writer for
// audit.class_list_entry_changes (#2382).
type ClassListEntryChangeRepository struct {
	runtime Runtime
}

// NewClassListEntryChangeRepository creates the audit writer.
func NewClassListEntryChangeRepository(runtime Runtime) auditModels.ClassListEntryChangeRepository {
	return &ClassListEntryChangeRepository{runtime: requireRuntime(runtime)}
}

// Create inserts one change row.
func (r *ClassListEntryChangeRepository) Create(ctx context.Context, change *auditModels.ClassListEntryChange) error {
	if change == nil {
		return fmt.Errorf("class list entry change audit event is required")
	}
	if err := change.Validate(); err != nil {
		return fmt.Errorf("invalid class list entry change audit event: %w", err)
	}
	return NewAppender(r.runtime).Append(ctx, change)
}

// ListByEntryID returns the trail of one entry, newest first.
func (r *ClassListEntryChangeRepository) ListByEntryID(ctx context.Context, entryID int64) ([]*auditModels.ClassListEntryChange, error) {
	var changes []*auditModels.ClassListEntryChange
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&changes).
		ModelTableExpr(`audit.class_list_entry_changes AS "class_list_entry_change"`).
		Where(`"class_list_entry_change".entry_id = ?`, entryID).
		OrderExpr(`"class_list_entry_change".occurred_at DESC, "class_list_entry_change".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list class list entry changes: %w", err)
	}
	return changes, nil
}
