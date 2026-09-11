package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewDocumentCleanup(db *bun.DB, now func() time.Time) (*workforce.DocumentCleanup, error) {
	if db == nil {
		return nil, errors.New("document cleanup: database is required")
	}
	if now == nil {
		now = time.Now
	}
	store := postgres.New(databaseRuntime(db))
	service := application.NewDocumentCleanup(store, transaction{lock: store.AcquireXactLock}, now)
	return workforce.NewDocumentCleanup(service.Enqueue,
		func(ctx context.Context, limit int) ([]workforce.DocumentCleanupClaim, error) {
			if _, active := tenant.TransactionFromContext(ctx); active {
				return nil, errors.New("document cleanup: claim must commit independently")
			}
			values, err := service.Claim(ctx, limit)
			if err != nil {
				return nil, err
			}
			result := make([]workforce.DocumentCleanupClaim, 0, len(values))
			for _, value := range values {
				result = append(result, workforce.DocumentCleanupClaim(value))
			}
			return result, nil
		},
		func(ctx context.Context, claim workforce.DocumentCleanupClaim, success bool) (bool, error) {
			return service.Finish(ctx, domain.DocumentCleanupClaim(claim), success)
		},
		func(ctx context.Context) (workforce.DocumentCleanupBacklog, error) {
			result, err := service.Backlog(ctx)
			return workforce.DocumentCleanupBacklog(result), err
		}), nil
}
