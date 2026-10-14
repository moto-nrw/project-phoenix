package api

import (
	"context"
	"log/slog"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// timeTrackingTestGraph is what a test module supplies beyond the
// work-session repositories to compose the Workforce time-tracking side the
// way the factory composes it (#3413).
type timeTrackingTestGraph struct {
	Audits        services.TimeTrackingAudits
	Settings      settingsCompose.SettingsService
	People        services.TimeTrackingStaffLookup
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
	return services.NewTimeTracking(services.TimeTrackingPortDependencies{
		Workforce: repos.Workforce, StaffAbsences: repos.StaffAbsence, AbsenceAudits: workforceCompose.NewStaffAbsenceAuditRepository(repos.Workforce),
		Audits: graph.Audits, Supervisions: repos.GroupSupervisor, SessionRecords: repos.ActiveGroup,
		Staff: repos.Staff, People: graph.People, StaffEmployment: workforceCompose.MustNewStaffEmployment(db.Pool()),
		WorkSchedules: repos.StaffWorkSchedule, WorkTimeModels: repos.WorkTimeModel, Holidays: graph.Holidays,
		Settings: graph.Settings, Payroll: payroll, Events: graph.Events, ShiftPlanSync: graph.ShiftPlanSync,
		DB: db.Pool(), Logger: graph.Logger, Today: graph.Today,
	})
}

// noHolidays is the calendar of a test graph that composes no School
// Calendar: no day loses its Soll.
type noHolidays struct{}

func (noHolidays) HolidayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return nil, nil
}
