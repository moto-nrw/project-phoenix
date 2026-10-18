//go:build testhooks

package api

import (
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TimetableScenarioTestModule is the Timetable side of the timetable
// scenario suites: the owner's module and its data cleanup.
type TimetableScenarioTestModule struct {
	TimetableTestModule
	TimetableCleanup timetable.TimetableCleanup
}

// NewTimetableScenarioTestModule composes the Timetable owner over db and
// returns it with the live graph the scenarios run beside (presence, people,
// caller context and settings) and the School Calendar the routes read.
func NewTimetableScenarioTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, clocks ...func() time.Time) (TimetableScenarioTestModule, ActiveTestModule, schoolcalendar.Calendar, error) {
	timetable, calendar, _, err := NewTimetableTestModule(db, unit, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, ActiveTestModule{}, nil, err
	}
	live, err := NewActiveTestModule(db, unit, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, ActiveTestModule{}, nil, err
	}
	liveSettings := NewActiveTestSettings(db, unit, clocks...)
	r, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return TimetableScenarioTestModule{}, ActiveTestModule{}, nil, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return TimetableScenarioTestModule{}, ActiveTestModule{}, nil, err
	}
	cleanup, err := NewTimetableCleanup(r.Timetable, NewDataDeletionTestRepository(db, command),
		NewTimetableDeviationAudit(db).DeviationEvent, liveSettings.Settings, slog.Default(), OptionalClock(clocks))
	if err != nil {
		return TimetableScenarioTestModule{}, ActiveTestModule{}, nil, err
	}
	return TimetableScenarioTestModule{TimetableTestModule: timetable, TimetableCleanup: cleanup}, live, calendar, nil
}
