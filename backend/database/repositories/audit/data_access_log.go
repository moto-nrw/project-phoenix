package audit

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
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
