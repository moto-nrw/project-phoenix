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

// NewParentConferences binds the Elternsprechtag days and bookings (#4801,
// ADR 0142) to the same runtime.
func NewParentConferences(db *bun.DB) schoolcalendar.ParentConferences {
	runtime := PersistenceRuntimeFor(db)
	return postgres.ParentConferences{Store: postgres.New(postgres.NewFeedHistoryDatabase(runtime.Database, runtime.TenantID))}
}
