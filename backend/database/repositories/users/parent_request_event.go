package users

import (
	"context"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

const parentRequestEventTable = "users.parent_request_events"

// parentRequestEventDefaultLimit bounds an unbounded student query so one
// child with a long history cannot pull an unpaged table scan into a request.
const parentRequestEventDefaultLimit = 200

type ParentRequestEventRepository struct {
	runtime Runtime
}

func NewParentRequestEventRepository(runtime Runtime) userModels.ParentRequestEventRepository {
	return &ParentRequestEventRepository{runtime: requireRuntime(runtime)}
}

// Create overrides the generic insert so created_at uses clock_timestamp():
// several events can be written inside one transaction and their real order
// is what the history shows.
func (r *ParentRequestEventRepository) Create(ctx context.Context, event *userModels.ParentRequestEvent) error {
	if event == nil {
		return fmt.Errorf("parent request event cannot be nil")
	}
	ensureTenantID(ctx, r.runtime, event)
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	if _, err := r.runtime.DB(ctx).NewInsert().Model(event).
		ModelTableExpr(parentRequestEventTable).
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx); err != nil {
		return &userModels.DatabaseError{Op: "create parent request event", Err: translateNotFound(err)}
	}
	return nil
}

// ListForRequest returns one request's events oldest first — the order the
// history is read in.
func (r *ParentRequestEventRepository) ListForRequest(
	ctx context.Context,
	requestType string,
	requestID int64,
) ([]*userModels.ParentRequestEvent, error) {
	rows := make([]*userModels.ParentRequestEvent, 0)
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_request_events AS "parent_request_event"`).
		Where(`"parent_request_event".request_type = ?`, requestType).
		Where(`"parent_request_event".request_id = ?`, requestID).
		OrderExpr(`"parent_request_event".id`)
	query = withTenantFilter(ctx, r.runtime, query, "parent_request_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &userModels.DatabaseError{Op: "list parent request events", Err: translateNotFound(err)}
	}
	return rows, nil
}

// ListForStudent returns one child's events newest first.
func (r *ParentRequestEventRepository) ListForStudent(
	ctx context.Context,
	studentID int64,
	limit int,
) ([]*userModels.ParentRequestEvent, error) {
	if limit <= 0 || limit > parentRequestEventDefaultLimit {
		limit = parentRequestEventDefaultLimit
	}
	rows := make([]*userModels.ParentRequestEvent, 0)
	query := r.runtime.DB(ctx).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_request_events AS "parent_request_event"`).
		Where(`"parent_request_event".student_id = ?`, studentID).
		OrderExpr(`"parent_request_event".id DESC`).
		Limit(limit)
	query = withTenantFilter(ctx, r.runtime, query, "parent_request_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &userModels.DatabaseError{Op: "list student parent request events", Err: translateNotFound(err)}
	}
	return rows, nil
}
