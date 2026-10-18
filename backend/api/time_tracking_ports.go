package api

import (
	"context"
	"log/slog"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// timeTrackingStaffRecords are the retained staff reads and the schedule
// binding write the time-tracking ports need from the People Directory.
type timeTrackingStaffRecords interface {
	workSessionStaffRecords
	staffListRecords
}

// timeTrackingStaffLookup resolves the staff record a schedule update is
// written for.
type timeTrackingStaffLookup interface {
	GetStaffByID(ctx context.Context, id int64) (*users.Staff, error)
}

// timeTrackingSettings is the tenant-settings surface the time-tracking
// services read through Student Presence's named questions.
type timeTrackingSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// timeTrackingAudits are the four audit trails the time-tracking services
// write and read.
type timeTrackingAudits struct {
	WorkSessionEdit      workSessionEditTrail
	TimeTrackingDeletion timeTrackingDeletionTrail
	DataAccessLog        interface {
		Create(context.Context, *audit.DataAccessLog) error
	}
	TimeTrackingAuditLog timeTrackingAuditFeed
}

// timeTrackingPortDependencies are the owners the root binds the Workforce
// time-tracking ports over (#3413).
type timeTrackingPortDependencies struct {
	Workforce workforceModule.Capability
	// Bound are the ports the caller binds before timeTrackingPorts runs:
	// the staff absences and their audit, the work-session staff, the
	// supervisions and session records, the holidays, the events, the
	// absence mail and the shift-plan sync. timeTrackingPorts sets every
	// other port and leaves these alone.
	Bound           workforceCompose.TimeTrackingPorts
	Audits          timeTrackingAudits
	Staff           timeTrackingStaffRecords
	People          timeTrackingStaffLookup
	StaffEmployment workforceCompose.StaffEmploymentQuery
	WorkSchedules   interface {
		WorkSessionScheduleRecords
		WorkScheduleTargetRecords
	}
	WorkTimeModels interface {
		WorkSessionTimeModelRecords
		WorkTimeTargetModelRecords
	}
	Settings timeTrackingSettings
	Payroll  settingsCompose.PayrollStatusGetter
	DB       transactionruntime.Database
	Logger   *slog.Logger
	Today    func() timezone.Date
}

// newTimeTracking composes the Workforce time-tracking side over the root's
// port bindings.
func newTimeTracking(deps timeTrackingPortDependencies) (*workforceCompose.TimeTracking, error) {
	return workforceCompose.NewTimeTracking(deps.Workforce, timeTrackingPorts(deps), staffScheduleBindings(deps.People))
}

// timeTrackingPorts binds the time-tracking services' consumer-owned ports to
// the retained repositories, audit trails, settings and renderers.
func timeTrackingPorts(deps timeTrackingPortDependencies) workforceCompose.TimeTrackingPorts {
	ports := deps.Bound
	ports.EditAudit = NewWorkSessionAudit(deps.Audits.WorkSessionEdit)
	ports.DeletionAudit = NewTimeTrackingDeletionAudit(deps.Audits.TimeTrackingDeletion)
	ports.DataAccessAudit = NewTimeTrackingDataAccessAudit(deps.Audits.DataAccessLog)
	ports.AuditReader = NewTimeTrackingAuditReader(deps.Audits.TimeTrackingAuditLog)
	ports.StaffSchedules = workforceCompose.StaffScheduleAssignments(deps.StaffEmployment)
	ports.MonthCloseStaff, ports.OverviewStaff = MonthCloseStaff(deps.Staff), OverviewStaff(deps.Staff)
	ports.ExportStaff, ports.StaffNames = TimeExportStaff(deps.Staff), StaffDisplayNames(deps.Staff)
	ports.Schedules = NewWorkSessionSchedules(deps.WorkSchedules)
	ports.TimeModels = NewWorkSessionTimeModels(deps.WorkTimeModels)
	ports.ScheduleTargets = NewWorkScheduleTargets(deps.WorkSchedules)
	ports.TargetModels = NewWorkTimeTargetModels(deps.WorkTimeModels)
	ports.Shifts = workforceCompose.NewTimeTrackingShifts(deps.Workforce)
	ports.Settings = settingsCompose.PresenceSettings(deps.Settings)
	ports.Payroll = PayrollExportSettings{Source: deps.Payroll}
	ports.RenderPDF, ports.RenderWorkbook = RenderTimeTrackingPDF, RenderTimeTrackingWorkbook
	ports.Logger, ports.Today = deps.Logger, deps.Today
	if pool := deps.DB.Pool(); pool != nil {
		ports.DB = pool
	}
	return ports
}

// staffScheduleBindings reads the work time model and rotation anchor a
// schedule update starts from.
func staffScheduleBindings(people timeTrackingStaffLookup) workforceCompose.StaffScheduleBindingLookup {
	return func(ctx context.Context, staffID int64) (workforceCompose.StaffScheduleBinding, error) {
		staff, err := people.GetStaffByID(ctx, staffID)
		if err != nil {
			return workforceCompose.StaffScheduleBinding{}, err
		}
		return workforceCompose.StaffScheduleBinding{ID: staff.ID, WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate}, nil
	}
}

// timeTrackingStaffRows are the staff rows the time-tracking ports and the
// absence request mails read.
type timeTrackingStaffRows interface {
	timeTrackingStaffRecords
	absenceEmailStaffRecords
}
