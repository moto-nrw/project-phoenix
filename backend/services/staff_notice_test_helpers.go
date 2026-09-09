package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/uptrace/bun"
)

// NewStaffNoticeTestService verdrahtet die Tagesinformationen (#2180, #2208)
// so wie die Factory: eigenes Repository, Kalenderzeiträume für das
// Wochenmuster und das echte Personenverzeichnis hinter der Bestätigungsliste.
// Adapter-Tests holen sich den Dienst hier, statt die Legacy-Factory zu bauen.
func NewStaffNoticeTestService(db *bun.DB) (schedule.StaffNoticeService, error) {
	timetable, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	return schedule.NewStaffNoticeService(schedule.StaffNoticeServiceConfig{
		Repo:    scheduleRepo.NewStaffNoticeRepository(db),
		Periods: timetable.CalendarPeriod,
		Names:   newStaffNoticeNameLookup(persons),
		Logger:  slog.Default(),
	}), nil
}
