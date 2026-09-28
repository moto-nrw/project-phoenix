package users

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

const familyProtectionTable = "users.student_family_protection_events"

type FamilyProtectionEventRepository struct {
	runtime Runtime
}

func NewFamilyProtectionEventRepository(runtime Runtime) userModels.FamilyProtectionEventRepository {
	return &FamilyProtectionEventRepository{runtime: requireRuntime(runtime)}
}

// Create overrides the generic insert so created_at uses clock_timestamp().
// Sharing and protection events live in separate tables, and visibility compares
// their real creation order even when both are written in one transaction.
func (r *FamilyProtectionEventRepository) Create(ctx context.Context, event *userModels.FamilyProtectionEvent) error {
	if event == nil {
		return fmt.Errorf("family protection event cannot be nil")
	}
	ensureTenantID(ctx, r.runtime, event)
	if _, err := r.runtime.DB(ctx).NewInsert().Model(event).
		ModelTableExpr(familyProtectionTable).
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx); err != nil {
		return &userModels.DatabaseError{Op: "create family protection event", Err: translateNotFound(err)}
	}
	return nil
}

// CurrentForStudents uses DISTINCT ON because the generic filter API cannot
// select the newest immutable event independently for each child.
func (r *FamilyProtectionEventRepository) CurrentForStudents(ctx context.Context, studentIDs []int64) (map[int64]*userModels.FamilyProtectionEvent, error) {
	result := make(map[int64]*userModels.FamilyProtectionEvent, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	var rows []*userModels.FamilyProtectionEvent
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.student_family_protection_events AS "family_protection_event"`).
		Where(`"family_protection_event".student_id IN (?)`, bun.List(studentIDs)).
		DistinctOn(`"family_protection_event".student_id`).
		OrderExpr(`"family_protection_event".student_id, "family_protection_event".id DESC`)
	query = withTenantFilter(ctx, r.runtime, query, "family_protection_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &userModels.DatabaseError{Op: "list current family protection", Err: translateNotFound(err)}
	}
	for _, row := range rows {
		result[row.StudentID] = row
	}
	return result, nil
}
