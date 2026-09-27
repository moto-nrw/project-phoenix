package users

import (
	"context"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

const parentRequestShareTable = "users.parent_request_share_events"

type ParentRequestShareEventRepository struct {
	runtime Runtime
}

func NewParentRequestShareEventRepository(runtime Runtime) userModels.ParentRequestShareEventRepository {
	return &ParentRequestShareEventRepository{runtime: requireRuntime(runtime)}
}

// Create overrides the generic insert so created_at uses clock_timestamp().
// Sharing and protection events live in separate tables, and visibility compares
// their real creation order even when both are written in one transaction.
func (r *ParentRequestShareEventRepository) Create(ctx context.Context, event *userModels.ParentRequestShareEvent) error {
	if event == nil {
		return fmt.Errorf("parent request share event cannot be nil")
	}
	ensureTenantID(ctx, r.runtime, event)
	if event.RecipientAccountIDs == nil {
		event.RecipientAccountIDs = []int64{}
	}
	if _, err := r.runtime.DB(ctx).NewInsert().Model(event).
		ModelTableExpr(parentRequestShareTable).
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx); err != nil {
		return &userModels.DatabaseError{Op: "create parent request share event", Err: translateNotFound(err)}
	}
	return nil
}

// CurrentForStudent uses DISTINCT ON because the generic filter API cannot
// select the newest immutable event for each request type and request ID.
func (r *ParentRequestShareEventRepository) CurrentForStudent(ctx context.Context, studentID int64) ([]*userModels.ParentRequestShareEvent, error) {
	rows := make([]*userModels.ParentRequestShareEvent, 0)
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_request_share_events AS "parent_request_share_event"`).
		Where(`"parent_request_share_event".student_id = ?`, studentID).
		DistinctOn(`"parent_request_share_event".request_type, "parent_request_share_event".request_id`).
		OrderExpr(`"parent_request_share_event".request_type, "parent_request_share_event".request_id, "parent_request_share_event".id DESC`)
	query = withTenantFilter(ctx, r.runtime, query, "parent_request_share_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &userModels.DatabaseError{Op: "list current parent request shares", Err: translateNotFound(err)}
	}
	return rows, nil
}
