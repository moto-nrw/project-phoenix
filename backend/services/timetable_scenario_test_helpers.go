package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/active"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TimetableScenarioTestModule struct {
	TimetableTestModule
	Active           active.Service
	Users            users.PersonService
	UserContext      usercontext.UserContextService
	Settings         config.SettingsService
	TimetableCleanup schedule.TimetableCleanupService
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
	cleanup := schedule.NewTimetableCleanupService(r.ActivityInstance, r.ActivityException, r.InstanceStudent,
		repositories.NewDataDeletionTestRepository(db, command), r.DeviationEvent, live.Settings, slog.Default(), optionalClock(clocks))
	return TimetableScenarioTestModule{TimetableTestModule: timetable, Active: live.Active, Users: live.Users, UserContext: live.UserContext, Settings: live.Settings, TimetableCleanup: cleanup}, nil
}
