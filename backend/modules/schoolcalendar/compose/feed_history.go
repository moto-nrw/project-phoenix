package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

// NewFeedHistory binds feed retention to School Calendar's tenant runtime.
func NewFeedHistory(db *bun.DB) schoolcalendar.FeedHistory {
	runtime := PersistenceRuntimeFor(db)
	return postgres.NewNativeFeedHistory(postgres.New(postgres.NewFeedHistoryDatabase(runtime.Database, runtime.TenantID)))
}
