package api

import (
	"context"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
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

// timeTrackingWorkSchedules are the staff work schedules the session and
// target ports read.
type timeTrackingWorkSchedules interface {
	WorkSessionScheduleRecords
	WorkScheduleTargetRecords
}

// timeTrackingWorkTimeModels are the work time models the session and target
// ports read.
type timeTrackingWorkTimeModels interface {
	WorkSessionTimeModelRecords
	WorkTimeTargetModelRecords
}

// timeTrackingPorts binds the time-tracking services' consumer-owned ports to
// the retained repositories, audit trails, settings and renderers.
func timeTrackingPorts(
	workforce workforceModule.Capability, bound workforceCompose.TimeTrackingPorts, audits timeTrackingAudits,
	staff timeTrackingStaffRecords, employment workforceCompose.StaffEmploymentQuery,
	schedules timeTrackingWorkSchedules, timeModels timeTrackingWorkTimeModels,
	settings timeTrackingSettings, payroll settingsCompose.PayrollStatusGetter, db transactionruntime.Database,
) workforceCompose.TimeTrackingPorts {
	ports := bound
	ports.EditAudit = NewWorkSessionAudit(audits.WorkSessionEdit)
	ports.DeletionAudit = NewTimeTrackingDeletionAudit(audits.TimeTrackingDeletion)
	ports.DataAccessAudit = NewTimeTrackingDataAccessAudit(audits.DataAccessLog)
	ports.AuditReader = NewTimeTrackingAuditReader(audits.TimeTrackingAuditLog)
	ports.StaffSchedules = workforceCompose.StaffScheduleAssignments(employment)
	ports.MonthCloseStaff, ports.OverviewStaff = MonthCloseStaff(staff), OverviewStaff(staff)
	ports.ExportStaff, ports.StaffNames = TimeExportStaff(staff), StaffDisplayNames(staff)
	ports.Schedules = NewWorkSessionSchedules(schedules)
	ports.TimeModels = NewWorkSessionTimeModels(timeModels)
	ports.ScheduleTargets = NewWorkScheduleTargets(schedules)
	ports.TargetModels = NewWorkTimeTargetModels(timeModels)
	ports.Shifts = workforceCompose.NewTimeTrackingShifts(workforce)
	ports.Settings = settingsCompose.PresenceSettings(settings)
	ports.Payroll = PayrollExportSettings{Source: payroll}
	ports.RenderPDF, ports.RenderWorkbook = RenderTimeTrackingPDF, RenderTimeTrackingWorkbook
	if pool := db.Pool(); pool != nil {
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
