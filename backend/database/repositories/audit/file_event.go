package audit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

const fileEventTableExpr = `audit.file_events AS "file_event"`

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
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&rows).
		ModelTableExpr(fileEventTableExpr).
		OrderExpr(`"file_event".created_at DESC, "file_event".id DESC`).
		Limit(limit)
	if tenantID := runtimeTenantID(ctx, r.runtime); tenantID > 0 {
		query = query.Where(`"file_event".tenant_id = ?`, tenantID)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, wrapDatabase("list file events", err)
	}
	return rows, nil
}
