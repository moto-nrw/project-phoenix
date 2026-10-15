package api

import (
	"context"
	"log/slog"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// timeTrackingTestGraph is what a test module supplies beyond the
// work-session repositories to compose the Workforce time-tracking side the
// way the service graph composes it (#3413).
type timeTrackingTestGraph struct {
	Audits        timeTrackingAudits
	Settings      settingsCompose.SettingsService
	People        timeTrackingStaffLookup
	Holidays      workforceCompose.HolidayDatesReader
	Events        workforceCompose.EventPublisher
	ShiftPlanSync workforceCompose.ShiftPlanSyncer
	Logger        *slog.Logger
	Today         func() timezone.Date
}

func newTimeTrackingTestModule(db transactionruntime.Database, repos WorkSessionTestRepositories, graph timeTrackingTestGraph) (*workforceCompose.TimeTracking, error) {
	payroll := settingsCompose.NewPayrollStatusService(graph.Settings, func(ctx context.Context) (int, int, error) {
		staff, err := repos.Staff.List(ctx, nil)
		if err != nil {
			return 0, 0, err
		}
		withoutPersonnelNumber := 0
		for _, member := range staff {
			if member.PersonnelNumber == nil || *member.PersonnelNumber == "" {
				withoutPersonnelNumber++
			}
		}
		return len(staff), withoutPersonnelNumber, nil
	})
	return newTimeTracking(timeTrackingPortDependencies{
		Workforce: repos.Workforce,
		Bound: workforceCompose.TimeTrackingPorts{
			Absences: repos.StaffAbsence, AbsenceAudits: workforceCompose.NewStaffAbsenceAuditRepository(repos.Workforce),
			Supervisions: repos.GroupSupervisor, SessionRecords: repos.ActiveGroup, Holidays: graph.Holidays,
			Events: graph.Events, ShiftPlanSync: graph.ShiftPlanSync,
		},
		Audits: graph.Audits,
		Staff:  repos.Staff, People: graph.People, StaffEmployment: workforceCompose.MustNewStaffEmployment(db.Pool()),
		WorkSchedules: repos.StaffWorkSchedule, WorkTimeModels: repos.WorkTimeModel,
		Settings: graph.Settings, Payroll: payroll,
		DB: db, Logger: graph.Logger, Today: graph.Today,
	})
}

// noHolidays is the calendar of a test graph that composes no School
// Calendar: no day loses its Soll.
type noHolidays struct{}

func (noHolidays) HolidayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return nil, nil
}
