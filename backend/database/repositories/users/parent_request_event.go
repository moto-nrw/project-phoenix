package users

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

const parentRequestEventTable = "users.parent_request_events"

// parentRequestEventDefaultLimit bounds an unbounded student query so one
// child with a long history cannot pull an unpaged table scan into a request.
const parentRequestEventDefaultLimit = 200

type ParentRequestEventRepository struct {
	*base.Repository[*userModels.ParentRequestEvent]
}

func NewParentRequestEventRepository(db *bun.DB) userModels.ParentRequestEventRepository {
	repo := base.NewRepository[*userModels.ParentRequestEvent](db, parentRequestEventTable, "ParentRequestEvent")
	repo.TenantScoped = true
	return &ParentRequestEventRepository{Repository: repo}
}

// Create overrides the generic insert so created_at uses clock_timestamp():
// several events can be written inside one transaction and their real order
// is what the history shows.
func (r *ParentRequestEventRepository) Create(ctx context.Context, event *userModels.ParentRequestEvent) error {
	if event == nil {
		return fmt.Errorf("parent request event cannot be nil")
	}
	base.EnsureTenantID(ctx, event)
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	if _, err := base.GetDB(ctx, r.DB).NewInsert().Model(event).
		ModelTableExpr(parentRequestEventTable).
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create parent request event", Err: base.TranslateNotFound(err)}
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
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_request_events AS "parent_request_event"`).
		Where(`"parent_request_event".request_type = ?`, requestType).
		Where(`"parent_request_event".request_id = ?`, requestID).
		OrderExpr(`"parent_request_event".id`)
	query = base.WithTenantFilter(ctx, query, "parent_request_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent request events", Err: base.TranslateNotFound(err)}
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
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_request_events AS "parent_request_event"`).
		Where(`"parent_request_event".student_id = ?`, studentID).
		OrderExpr(`"parent_request_event".id DESC`).
		Limit(limit)
	query = base.WithTenantFilter(ctx, query, "parent_request_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student parent request events", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}
