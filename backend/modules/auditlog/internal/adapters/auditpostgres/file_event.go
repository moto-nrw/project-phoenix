package audit

import (
	"context"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
)

const fileEventTableExpr = `audit.file_event_ledger AS "file_event"`

type fileEventRepository struct {
	runtime Runtime
}

// NewFileEventRepository creates the append-only file storage trail
// repository (#2596).
func NewFileEventRepository(runtime Runtime) audit.FileEventRepository {
	return &fileEventRepository{runtime: requireRuntime(runtime)}
}

func (r *fileEventRepository) Create(ctx context.Context, event *audit.FileEvent) error {
	return NewAppender(r.runtime).Append(ctx, event)
}

func (r *fileEventRepository) ListRecent(ctx context.Context, limit int) ([]*audit.FileEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []*audit.FileEvent
	ambientDB1, databaseErr1 := runtimeDB(ctx, r.runtime)
	if databaseErr1 != nil {
		return *new([]*audit.FileEvent), databaseErr1
	}
	query := ambientDB1.NewSelect().
		Model(&rows).
		ModelTableExpr(fileEventTableExpr).
		OrderExpr(`"file_event".created_at DESC, "file_event".id DESC`).
		Limit(int64(limit))
	query = tenantQuery(ctx, r.runtime, query, "file_event")
	if err := query.Scan(ctx); err != nil {
		return nil, wrapDatabase("list file events", err)
	}
	return rows, nil
}
