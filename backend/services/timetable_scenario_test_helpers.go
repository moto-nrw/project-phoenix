package services

import (
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type TimetableScenarioTestModule struct {
	TimetableTestModule
	Active           studentpresence.Presence
	Users            *peopleCompose.PersonDirectory
	UserContext      *CallerRows
	Settings         settingsCompose.SettingsService
	TimetableCleanup timetable.TimetableCleanup
	// People serves the People port of the timetable routes from Users, as
	// the root binds it.
	People TimetablePeople
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
	r, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	cleanup, err := newTimetableCleanup(timetableRetentionInputs{
		Owner: r.Timetable, Deletions: NewDataDeletionTestRepository(db, command),
		Deviations: r.DeviationEvent, Settings: live.Settings, Logger: slog.Default(), Clock: optionalClock(clocks),
	})
	if err != nil {
		return TimetableScenarioTestModule{}, err
	}
	return TimetableScenarioTestModule{TimetableTestModule: timetable, Active: live.Active, Users: live.Users, UserContext: live.UserContext, Settings: live.Settings, TimetableCleanup: cleanup, People: NewTimetablePeople(live.Users)}, nil
}
