package services

import (
	"context"
	"log/slog"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// timeTrackingStaffRecords are the retained staff reads and the schedule
// binding write the time-tracking ports need from the People Directory.
type timeTrackingStaffRecords interface {
	workSessionStaffRecords
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

// TimeTrackingStaffLookup resolves the staff record a schedule update is
// written for.
type TimeTrackingStaffLookup interface {
	GetStaffByID(ctx context.Context, id int64) (*users.Staff, error)
}

// TimeTrackingAudits are the four audit trails the time-tracking services
// write and read.
type TimeTrackingAudits struct {
	WorkSessionEdit      audit.WorkSessionEditRepository
	TimeTrackingDeletion audit.TimeTrackingDeletionRepository
	DataAccessLog        interface {
		Create(context.Context, *audit.DataAccessLog) error
	}
	TimeTrackingAuditLog audit.TimeTrackingAuditLogRepository
}

// TimeTrackingPortDependencies are the owners the root binds the Workforce
// time-tracking ports over (#3413).
type TimeTrackingPortDependencies struct {
	Workforce       workforce.Capability
	StaffAbsences   workforceCompose.StaffAbsenceRepository
	AbsenceAudits   workforceCompose.StaffAbsenceAuditRepository
	Audits          TimeTrackingAudits
	Supervisions    workforceCompose.GroupSupervisorRepository
	SessionRecords  workforceCompose.GroupRepository
	Staff           timeTrackingStaffRecords
	People          TimeTrackingStaffLookup
	StaffEmployment staffScheduleRecords
	WorkSchedules   interface {
		WorkSessionScheduleRecords
		WorkScheduleTargetRecords
	}
	WorkTimeModels interface {
		WorkSessionTimeModelRecords
		WorkTimeTargetModelRecords
	}
	Holidays workforceCompose.HolidayDatesReader
	Settings settingsSource
	Payroll  settingsCompose.PayrollStatusGetter
	Events   workforceCompose.EventPublisher
	// AbsenceEmail and ShiftPlanSync are optional.
	AbsenceEmail  *workforceCompose.AbsenceEmailDeps
	ShiftPlanSync workforceCompose.ShiftPlanSyncer
	DB            *bun.DB
	Logger        *slog.Logger
	Today         func() timezone.Date
}

// NewTimeTracking composes the Workforce time-tracking side over the root's
// port bindings.
func NewTimeTracking(deps TimeTrackingPortDependencies) (*workforceCompose.TimeTracking, error) {
	return workforceCompose.NewTimeTracking(deps.Workforce, TimeTrackingPorts(deps), staffScheduleBindings(deps.People))
}

// TimeTrackingPorts binds the time-tracking services' consumer-owned ports
// to the retained repositories, audit trails, settings and renderers.
func TimeTrackingPorts(deps TimeTrackingPortDependencies) workforceCompose.TimeTrackingPorts {
	ports := workforceCompose.TimeTrackingPorts{
		Absences: deps.StaffAbsences, AbsenceAudits: deps.AbsenceAudits,
		EditAudit:       NewWorkSessionAudit(deps.Audits.WorkSessionEdit),
		DeletionAudit:   NewTimeTrackingDeletionAudit(deps.Audits.TimeTrackingDeletion),
		DataAccessAudit: NewTimeTrackingDataAccessAudit(deps.Audits.DataAccessLog),
		AuditReader:     NewTimeTrackingAuditReader(deps.Audits.TimeTrackingAuditLog),
		Supervisions:    deps.Supervisions, SessionRecords: deps.SessionRecords,
		Staff:           WorkSessionStaff(deps.Staff, deps.StaffEmployment),
		StaffSchedules:  StaffScheduleAssignments(deps.StaffEmployment),
		MonthCloseStaff: MonthCloseStaff(deps.Staff), OverviewStaff: OverviewStaff(deps.Staff),
		ExportStaff: TimeExportStaff(deps.Staff), StaffNames: StaffDisplayNames(deps.Staff),
		Schedules:       NewWorkSessionSchedules(deps.WorkSchedules),
		TimeModels:      NewWorkSessionTimeModels(deps.WorkTimeModels),
		ScheduleTargets: NewWorkScheduleTargets(deps.WorkSchedules),
		TargetModels:    NewWorkTimeTargetModels(deps.WorkTimeModels),
		Shifts:          NewTimeTrackingShifts(deps.Workforce),
		Holidays:        deps.Holidays,
		Settings:        PresenceSettings(deps.Settings),
		Payroll:         PayrollExportSettings{Source: deps.Payroll},
		RenderPDF:       RenderTimeTrackingPDF, RenderWorkbook: RenderTimeTrackingWorkbook,
		Events: deps.Events, AbsenceEmail: deps.AbsenceEmail, Logger: deps.Logger, Today: deps.Today,
	}
	// A typed nil in an interface field is not nil, so only a set collaborator
	// is forwarded.
	if deps.ShiftPlanSync != nil {
		ports.ShiftPlanSync = deps.ShiftPlanSync
	}
	if deps.DB != nil {
		ports.DB = deps.DB
	}
	return ports
}

// staffScheduleBindings reads the work time model and rotation anchor a
// schedule update starts from.
func staffScheduleBindings(people TimeTrackingStaffLookup) workforceCompose.StaffScheduleBindingLookup {
	return func(ctx context.Context, staffID int64) (workforceCompose.StaffScheduleBinding, error) {
		staff, err := people.GetStaffByID(ctx, staffID)
		if err != nil {
			return workforceCompose.StaffScheduleBinding{}, err
		}
		return workforceCompose.StaffScheduleBinding{ID: staff.ID, WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate}, nil
	}
}

// workPresence serves today's work status map, the one read the reminder
// scheduler and the staff recipient resolver take from the work sessions.
type workPresence struct{ capability workforce.Capability }

// WorkPresence binds GetTodayPresenceMap to the Workforce capability.
func WorkPresence(capability workforce.Capability) interface {
	GetTodayPresenceMap(context.Context) (map[int64]string, error)
} {
	return workPresence{capability: capability}
}

func (p workPresence) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	return p.capability.WorkPresenceMap(ctx)
}
