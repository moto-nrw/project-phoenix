package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TimetableScenarioTestModule struct {
	TimetableTestModule
	Active           studentpresence.Presence
	Users            users.PersonService
	UserContext      *repositories.CallerRows
	Settings         config.SettingsService
	TimetableCleanup timetable.TimetableCleanup
}

func NewTimetableScenarioTestModule(db *bun.DB, unit tenant.UnitOfWork, clocks ...func() time.Time) (TimetableScenarioTestModule, error) {
	timetable, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	live, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	r, err := repositories.NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	cleanup, err := newTimetableCleanup(timetableRetentionInputs{
		Owner: r.Timetable, Deletions: repositories.NewDataDeletionTestRepository(db, command),
		Deviations: r.DeviationEvent, Settings: live.Settings, Logger: slog.Default(), Clock: optionalClock(clocks),
	})
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	return TimetableScenarioTestModule{TimetableTestModule: timetable, Active: live.Active, Users: live.Users, UserContext: live.UserContext, Settings: live.Settings, TimetableCleanup: cleanup}, nil
}
