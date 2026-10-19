package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// NewFeedHistory binds feed retention to School Calendar's tenant runtime.
func NewFeedHistory(db *bun.DB) schoolcalendar.FeedHistory {
	runtime := PersistenceRuntimeFor(db)
	return postgres.NewNativeFeedHistory(postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		tx, err := runtime.Database(ctx)
		return tx, runtime.TenantID(ctx), err
	}))
}
