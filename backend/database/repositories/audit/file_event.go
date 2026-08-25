package audit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const fileEventTableExpr = `audit.file_events AS "file_event"`

type fileEventRepository struct {
	*base.Repository[*audit.FileEvent]
	db *bun.DB
}

// NewFileEventRepository creates the append-only file storage trail
// repository (#2596).
func NewFileEventRepository(db *bun.DB) audit.FileEventRepository {
	repo := base.NewRepository[*audit.FileEvent](db, "audit.file_events", "FileEvent")
	repo.TenantScoped = true
	return &fileEventRepository{Repository: repo, db: db}
}

func (r *fileEventRepository) Create(ctx context.Context, event *audit.FileEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, event)
	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(event).
		ModelTableExpr(fileEventTableExpr).
		Returning("*").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create file event", Err: err}
	}
	return nil
}

func (r *fileEventRepository) ListRecent(ctx context.Context, limit int) ([]*audit.FileEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []*audit.FileEvent
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(fileEventTableExpr).
		OrderExpr(`"file_event".created_at DESC, "file_event".id DESC`).
		Limit(limit)
	query = base.WithTenantFilter(ctx, query, "file_event")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list file events", Err: err}
	}
	return rows, nil
}
