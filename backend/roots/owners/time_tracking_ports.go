package owners

import (
	"context"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// TimeTrackingStaffRecords are the retained staff reads and the schedule
// binding write the time-tracking ports need from the People Directory.
type TimeTrackingStaffRecords interface {
	workSessionStaffRecords
	staffListRecords
}

// TimeTrackingStaffLookup resolves the staff record a schedule update is
// written for.
type TimeTrackingStaffLookup interface {
	StaffRows() peoplePorts.StaffRepository
}

// TimeTrackingSettings is the tenant-settings surface the time-tracking
// services read through Student Presence's named questions.
type TimeTrackingSettings interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// TimeTrackingAudits are the four audit trails the time-tracking services
// write and read.
type TimeTrackingAudits struct {
	WorkSessionEdit      workSessionEditTrail
	TimeTrackingDeletion timeTrackingDeletionTrail
	DataAccessLog        interface {
		Create(context.Context, *audit.DataAccessLog) error
	}
	TimeTrackingAuditLog timeTrackingAuditFeed
}

// TimeTrackingWorkSchedules are the staff work schedules the session and
// target ports read.
type TimeTrackingWorkSchedules interface {
	WorkSessionScheduleRecords
	WorkScheduleTargetRecords
}

// TimeTrackingWorkTimeModels are the work time models the session and target
// ports read.
type TimeTrackingWorkTimeModels interface {
	WorkSessionTimeModelRecords
	WorkTimeTargetModelRecords
}

// TimeTrackingPorts binds the time-tracking services' consumer-owned ports to
// the retained repositories, audit trails, settings and renderers.
func TimeTrackingPorts(
	workforce workforceModule.Capability, bound workforceCompose.TimeTrackingPorts, audits TimeTrackingAudits,
	staff TimeTrackingStaffRecords, employment workforceCompose.StaffEmploymentQuery,
	schedules TimeTrackingWorkSchedules, timeModels TimeTrackingWorkTimeModels,
	settings TimeTrackingSettings, payroll settingsCompose.PayrollStatusGetter, db transactionruntime.Database,
) workforceCompose.TimeTrackingPorts {
	ports := bound
	ports.EditAudit = NewWorkSessionAudit(audits.WorkSessionEdit)
	ports.DeletionAudit = NewTimeTrackingDeletionAudit(audits.TimeTrackingDeletion)
	ports.DataAccessAudit = NewTimeTrackingDataAccessAudit(audits.DataAccessLog)
	ports.AuditReader = NewTimeTrackingAuditReader(audits.TimeTrackingAuditLog)
	ports.StaffSchedules = workforceCompose.StaffScheduleAssignments(employment)
	ports.MonthCloseStaff, ports.OverviewStaff = MonthCloseStaff(staff), OverviewStaff(staff)
	ports.ExportStaff, ports.StaffNames = TimeExportStaff(staff), StaffDisplayNames(staff)
	ports.ExternalCaregivers = externalCaregiverLookup(staff)
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

// StaffScheduleBindings reads the work time model and rotation anchor a
// schedule update starts from.
func StaffScheduleBindings(people TimeTrackingStaffLookup) workforceCompose.StaffScheduleBindingLookup {
	return func(ctx context.Context, staffID int64) (workforceCompose.StaffScheduleBinding, error) {
		staff, err := people.StaffRows().FindByID(ctx, staffID)
		if err != nil {
			return workforceCompose.StaffScheduleBinding{}, err
		}
		return workforceCompose.StaffScheduleBinding{ID: staff.ID, WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate}, nil
	}
}
