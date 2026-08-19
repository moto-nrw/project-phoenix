package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// ClassListEntryChangeRepository is the append-only writer for
// audit.class_list_entry_changes (#2382).
type ClassListEntryChangeRepository struct {
	db *bun.DB
}

// NewClassListEntryChangeRepository creates the audit writer.
func NewClassListEntryChangeRepository(db *bun.DB) auditModels.ClassListEntryChangeRepository {
	return &ClassListEntryChangeRepository{db: db}
}

// Create inserts one change row.
func (r *ClassListEntryChangeRepository) Create(ctx context.Context, change *auditModels.ClassListEntryChange) error {
	if change == nil {
		return fmt.Errorf("class list entry change audit event is required")
	}
	if err := change.Validate(); err != nil {
		return fmt.Errorf("invalid class list entry change audit event: %w", err)
	}
	base.EnsureTenantID(ctx, change)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(change).
		ModelTableExpr(`audit.class_list_entry_changes AS "class_list_entry_change"`).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create class list entry change audit event: %w", err)
	}
	return nil
}

// ListByEntryID returns the trail of one entry, newest first.
func (r *ClassListEntryChangeRepository) ListByEntryID(ctx context.Context, entryID int64) ([]*auditModels.ClassListEntryChange, error) {
	var changes []*auditModels.ClassListEntryChange
	err := base.GetDB(ctx, r.db).NewSelect().
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
