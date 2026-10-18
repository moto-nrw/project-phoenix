//go:build testhooks

package api

import (
	"context"
	"log/slog"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// timeTrackingTestGraph is what a test module supplies beyond the
// work-session repositories and the tenant settings to compose the Workforce
// time-tracking side the way the service graph composes it (#3413).
type timeTrackingTestGraph struct {
	Audits        timeTrackingAudits
	People        timeTrackingStaffLookup
	Holidays      workforceCompose.HolidayDatesReader
	Events        workforceCompose.EventPublisher
	ShiftPlanSync workforceCompose.ShiftPlanSyncer
	Logger        *slog.Logger
	Today         func() timezone.Date
}

func newTimeTrackingTestModule(db transactionruntime.Database, repos WorkSessionTestRepositories, staffRows peoplePorts.StaffRepository, presence TimetablePresenceRows, settings settingsCompose.SettingsService, graph timeTrackingTestGraph, mail ...*workforceCompose.AbsenceEmailDeps) (*workforceCompose.TimeTracking, error) {
	var absenceEmail *workforceCompose.AbsenceEmailDeps
	if len(mail) > 0 {
		absenceEmail = mail[0]
	}
	payroll := settingsCompose.NewPayrollStatusService(settings, func(ctx context.Context) (int, int, error) {
		staff, err := staffRows.List(ctx, nil)
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
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	employment := workforceCompose.MustNewStaffEmployment(db.Pool())
	side, err := newTimeTracking(repos.Workforce, workforceCompose.TimeTrackingPorts{
		Absences: repos.StaffAbsence, AbsenceAudits: workforceCompose.NewStaffAbsenceAuditRepository(repos.Workforce),
		Staff:        WorkSessionStaff(staffRows, employment, membership),
		Supervisions: presence.GroupSupervisor, SessionRecords: presence.ActiveGroup, Holidays: graph.Holidays,
		Events: graph.Events, Logger: graph.Logger, Today: graph.Today, AbsenceEmail: absenceEmail,
	}, graph.Audits, staffRows, graph.People, employment, NewStaffWorkScheduleRepository(repos.Workforce), NewWorkTimeModelRepository(repos.Workforce),
		settings, payroll, db,
	)
	if err != nil {
		return nil, err
	}
	return side.WithSickCascade(graph.ShiftPlanSync)
}

// noSickCascade is the sick cascade of a test graph that composes no shift
// plans: a sick report leaves the plans untouched.
type noSickCascade struct{}

func (noSickCascade) MarkSickForRange(context.Context, workforceModule.SickCascadeInput) error {
	return nil
}

func (noSickCascade) ClearSickForRange(context.Context, workforceModule.SickCascadeInput) error {
	return nil
}

func (noSickCascade) ReconcileSickRange(context.Context, workforceModule.SickCascadeInput, workforceModule.SickCascadeInput) error {
	return nil
}

func (noSickCascade) ReassignSickStamps(context.Context, int64, int64) error { return nil }

// noHolidays is the calendar of a test graph that composes no School
// Calendar: no day loses its Soll.
type noHolidays struct{}

func (noHolidays) HolidayDates(context.Context, timezone.Date, timezone.Date) (map[timezone.Date]bool, error) {
	return nil, nil
}

// PayrollSettings is the settings read the payroll status resolves the
// Lohnarten and DATEV identifiers through.
type PayrollSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// NewPayrollExportSettings binds the payroll export settings to the Settings
// Platform's payroll status over the given settings and personnel counter,
// the way the service graph binds them, for suites that may not name the
// Settings Platform's services.
func NewPayrollExportSettings(settings PayrollSettings, countPersonnel func(context.Context) (int, int, error)) PayrollExportSettings {
	return PayrollExportSettings{Source: settingsCompose.NewPayrollStatusService(settings, countPersonnel)}
}

// newTimeTracking composes the Workforce time-tracking side (#3413) over the
// root's port bindings; the caller completes it with the sick cascade. bound
// are the ports the caller binds: the staff absences and their audit, the
// work-session staff, the supervisions and session records, the holidays,
// the events, the absence mail, the logger and the clock. timeTrackingPorts sets every
// other port over the remaining arguments and leaves these alone.
func newTimeTracking(
	workforce workforceModule.Capability, bound workforceCompose.TimeTrackingPorts, audits timeTrackingAudits,
	staff timeTrackingStaffRecords, people timeTrackingStaffLookup, employment workforceCompose.StaffEmploymentQuery,
	schedules timeTrackingWorkSchedules, timeModels timeTrackingWorkTimeModels,
	settings timeTrackingSettings, payroll settingsCompose.PayrollStatusGetter, db transactionruntime.Database,
) (*workforceCompose.WorkSessionSide, error) {
	ports := timeTrackingPorts(workforce, bound, audits, staff, employment, schedules, timeModels, settings, payroll, db)
	return workforceCompose.NewWorkSessionSide(workforce, ports, staffScheduleBindings(people))
}
