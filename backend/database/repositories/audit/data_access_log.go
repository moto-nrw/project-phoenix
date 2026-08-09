package audit

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// dataAccessLogRepository is an insert-only repository for audit.data_access_log.
// It satisfies audit.DataAccessLogRepository (interface declared in models/audit).
type dataAccessLogRepository struct {
	*base.Repository[*audit.DataAccessLog]
	db *bun.DB
}

// NewDataAccessLogRepository creates a new DataAccessLogRepository.
func NewDataAccessLogRepository(db *bun.DB) audit.DataAccessLogRepository {
	repo := base.NewRepository[*audit.DataAccessLog](db, "audit.data_access_log", "DataAccessLog")
	repo.TenantScoped = true
	return &dataAccessLogRepository{
		Repository: repo,
		db:         db,
	}
}

// ExistsSince implements audit.DataAccessLogRepository. Metadata values are
// matched as text (`metadata->>key = value`). The generic repository's
// tenant filter does not cover custom queries, so the tenant clause is added
// explicitly here (defense in depth next to RLS).
func (r *dataAccessLogRepository) ExistsSince(ctx context.Context, actorAccountID int64, resourceType string, metadata map[string]string, since time.Time) (bool, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*audit.DataAccessLog)(nil)).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where(`"data_access_log".actor_account_id = ?`, actorAccountID).
		Where(`"data_access_log".resource_type = ?`, resourceType).
		Where(`"data_access_log".accessed_at >= ?`, since)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"data_access_log".tenant_id = ?`, tenantID)
	}
	for key, value := range metadata {
		query = query.Where(`"data_access_log".metadata->>? = ?`, key, value)
	}
	return query.Exists(ctx)
}
