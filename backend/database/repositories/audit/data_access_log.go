package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

// dataAccessLogRepository is an insert-only repository for audit.data_access_log.
// It satisfies audit.DataAccessLogRepository (interface declared in models/audit).
type dataAccessLogRepository struct {
	runtime Runtime
}

// NewDataAccessLogRepository creates a new DataAccessLogRepository.
func NewDataAccessLogRepository(runtime Runtime) audit.DataAccessLogRepository {
	return &dataAccessLogRepository{runtime: requireRuntime(runtime)}
}

func (r *dataAccessLogRepository) Create(ctx context.Context, entry *audit.DataAccessLog) error {
	return NewAppender(r.runtime).Append(ctx, entry)
}

// ExistsSince implements audit.DataAccessLogRepository. Metadata values are
// matched as text (`metadata->>key = value`). The generic repository's
// tenant filter does not cover custom queries, so the tenant clause is added
// explicitly here (defense in depth next to RLS).
//
// A missing tenant is an error, never a wider query: this answers a
// deduplication question, so a foreign-tenant hit would silently swallow the
// GDPR row this caller is about to write.
func (r *dataAccessLogRepository) ExistsSince(ctx context.Context, actorAccountID int64, resourceType string, metadata map[string]string, since time.Time) (bool, error) {
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return false, fmt.Errorf("data access log dedupe requires a tenant context")
	}
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model((*audit.DataAccessLog)(nil)).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where(`"data_access_log".actor_account_id = ?`, actorAccountID).
		Where(`"data_access_log".resource_type = ?`, resourceType).
		Where(`"data_access_log".accessed_at >= ?`, since).
		Where(`"data_access_log".tenant_id = ?`, tenantID)
	for key, value := range metadata {
		query = query.Where(`"data_access_log".metadata->>? = ?`, key, value)
	}
	return query.Exists(ctx)
}
