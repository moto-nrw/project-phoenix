package repositories

import (
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	calendarRepo "github.com/moto-nrw/project-phoenix/database/repositories/calendar"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	calendarModels "github.com/moto-nrw/project-phoenix/models/calendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/uptrace/bun"
)

type CalendarFeedTestRepositories struct {
	StaffFeed authModels.StaffCalendarFeedTokenRepository
	Tombstone calendarModels.StaffFeedTombstoneRepository
}

func NewCalendarFeedTestRepositories(db *bun.DB) CalendarFeedTestRepositories {
	runtime := schoolCalendarCompose.PersistenceRuntimeFor(db)
	return CalendarFeedTestRepositories{StaffFeed: authRepo.NewStaffCalendarFeedTokenRepository(db),
		Tombstone: calendarRepo.NewStaffFeedTombstoneRepository(calendarRepo.Runtime{Database: runtime.Database, TenantID: runtime.TenantID})}
}
