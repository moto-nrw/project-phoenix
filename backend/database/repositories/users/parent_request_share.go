package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const parentRequestShareTable = "users.parent_request_share_events"

type ParentRequestShareEventRepository struct {
	db *bun.DB
}

func NewParentRequestShareEventRepository(db *bun.DB) userModels.ParentRequestShareEventRepository {
	return &ParentRequestShareEventRepository{db: db}
}

func (r *ParentRequestShareEventRepository) Create(ctx context.Context, event *userModels.ParentRequestShareEvent) error {
	if event == nil {
		return fmt.Errorf("parent request share event cannot be nil")
	}
	base.EnsureTenantID(ctx, event)
	if event.RecipientAccountIDs == nil {
		event.RecipientAccountIDs = []int64{}
	}
	if _, err := base.GetDB(ctx, r.db).NewInsert().Model(event).
		ModelTableExpr(parentRequestShareTable).
		Value("created_at", "clock_timestamp()").
		Value("updated_at", "clock_timestamp()").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create parent request share event", Err: err}
	}
	return nil
}

func (r *ParentRequestShareEventRepository) CurrentForStudent(ctx context.Context, studentID int64) ([]*userModels.ParentRequestShareEvent, error) {
	rows := make([]*userModels.ParentRequestShareEvent, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.parent_request_share_events AS "parent_request_share_event"`).
		Where(`"parent_request_share_event".student_id = ?`, studentID).
		DistinctOn(`"parent_request_share_event".request_type, "parent_request_share_event".request_id`).
		OrderExpr(`"parent_request_share_event".request_type, "parent_request_share_event".request_id, "parent_request_share_event".id DESC`)
	query = base.WithTenantFilter(ctx, query, "parent_request_share_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list current parent request shares", Err: err}
	}
	return rows, nil
}
