package users

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

const familyProtectionTable = "users.student_family_protection_events"

type FamilyProtectionEventRepository struct {
	*base.Repository[*userModels.FamilyProtectionEvent]
}

func NewFamilyProtectionEventRepository(db *bun.DB) userModels.FamilyProtectionEventRepository {
	repo := base.NewRepository[*userModels.FamilyProtectionEvent](db, familyProtectionTable, "FamilyProtectionEvent")
	repo.TenantScoped = true
	return &FamilyProtectionEventRepository{Repository: repo}
}

// Create overrides the generic insert so created_at uses clock_timestamp().
// Sharing and protection events live in separate tables, and visibility compares
// their real creation order even when both are written in one transaction.
func (r *FamilyProtectionEventRepository) Create(ctx context.Context, event *userModels.FamilyProtectionEvent) error {
	if event == nil {
		return fmt.Errorf("family protection event cannot be nil")
	}
	base.EnsureTenantID(ctx, event)
	if _, err := base.GetDB(ctx, r.DB).NewInsert().Model(event).
		ModelTableExpr(familyProtectionTable).
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create family protection event", Err: base.TranslateNotFound(err)}
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
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.student_family_protection_events AS "family_protection_event"`).
		Where(`"family_protection_event".student_id IN (?)`, bun.List(studentIDs)).
		DistinctOn(`"family_protection_event".student_id`).
		OrderExpr(`"family_protection_event".student_id, "family_protection_event".id DESC`)
	query = base.WithTenantFilter(ctx, query, "family_protection_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list current family protection", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.StudentID] = row
	}
	return result, nil
}
