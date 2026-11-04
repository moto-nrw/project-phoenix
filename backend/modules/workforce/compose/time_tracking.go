package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	timetracking "github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

// The time-tracking side of Workforce (#3413): the stamps, the session
// history and its edits, the absences and the vacation quota, the month
// balances and their close, the overview, the exports and the audit feed.
// The services live in modules/workforce/internal/application/timetracking;
// this file binds them to the Workforce capability rows, adapts them to the
// public contracts (time_tracking_*.go) and hands those to the root. The readers of other owners are consumer-owned ports declared
// by the time-tracking package and named here for the root.

// TimeTrackingPorts are the readers, writers and renderers the root binds.
type TimeTrackingPorts = timetracking.Ports

// The port vocabulary the root's bindings implement.
type (
	AbsenceEmailContact         = timetracking.AbsenceEmailContact
	AbsenceEmailDeps            = timetracking.AbsenceEmailDeps
	AbsenceEmailMessage         = timetracking.AbsenceEmailMessage
	DataAccessAudit             = timetracking.DataAccessAudit
	DataAccessEvent             = timetracking.DataAccessEvent
	DeletionAudit               = timetracking.DeletionAudit
	DeletionEvent               = timetracking.DeletionEvent
	EventPublisher              = timetracking.EventPublisher
	GroupRepository             = timetracking.GroupRepository
	GroupSupervisorRepository   = timetracking.GroupSupervisorRepository
	HolidayDatesReader          = timetracking.HolidayDatesReader
	MonthCloseStaffQuery        = timetracking.MonthCloseStaffQuery
	OverviewStaff               = timetracking.OverviewStaff
	OverviewStaffQuery          = timetracking.OverviewStaffQuery
	PayrollExportCategory       = timetracking.PayrollExportCategory
	PayrollExportConfiguration  = timetracking.PayrollExportConfiguration
	StaffDisplayNameQuery       = timetracking.StaffDisplayNameQuery
	StaffScheduleAssignment     = timetracking.StaffScheduleAssignment
	StaffScheduleBinding        = timetracking.StaffScheduleBinding
	StaffScheduleQuery          = timetracking.StaffScheduleQuery
	TimeExportStaff             = timetracking.TimeExportStaff
	TimeExportStaffQuery        = timetracking.TimeExportStaffQuery
	TimeTrackingAuditCursor     = timetracking.TimeTrackingAuditCursor
	TimeTrackingAuditEntry      = timetracking.TimeTrackingAuditEntry
	TimeTrackingAuditFilter     = timetracking.TimeTrackingAuditFilter
	TimeTrackingAuditReader     = timetracking.TimeTrackingAuditReader
	TimeTrackingCleanupPreview  = timetracking.TimeTrackingCleanupPreview
	TimeTrackingCleanupResult   = timetracking.TimeTrackingCleanupResult
	TimeTrackingCleanupService  = timetracking.TimeTrackingCleanupService
	TimeTrackingCleanupSettings = timetracking.Settings
	TimeTrackingCleanupStats    = timetracking.TimeTrackingCleanupStats
	TimeTrackingDeletionAudit   = timetracking.TimeTrackingDeletionAudit
	TimeTrackingDeletionEvent   = timetracking.TimeTrackingDeletionEvent
	TimeTrackingDocument        = timetracking.TimeTrackingDocument
	TimeTrackingShift           = timetracking.TimeTrackingShift
	WorkScheduleRow             = timetracking.WorkScheduleRow
	WorkScheduleTargets         = timetracking.WorkScheduleTargets
	WorkSessionAudit            = timetracking.WorkSessionAudit
	WorkSessionEdit             = timetracking.WorkSessionEdit
	WorkSessionStaff            = timetracking.WorkSessionStaff
	WorkSessionStaffName        = timetracking.WorkSessionStaffName
	WorkTimeTargetModel         = timetracking.WorkTimeTargetModel
	WorkTimeTemplate            = timetracking.WorkTimeTemplate
	WorkTimeTemplateEntry       = timetracking.WorkTimeTemplateEntry
)

// NewWorkSessionSide composes every time-tracking service but the absences;
// staff resolves the record a schedule update is written for.
func NewWorkSessionSide(capability workforce.Capability, ports TimeTrackingPorts, staff StaffScheduleBindingLookup) (*WorkSessionSide, error) {
	if capability == nil || staff == nil {
		return nil, errors.New("time tracking: the Workforce capability and the staff lookup are required")
	}
	clock, err := timetracking.NewClock(timetracking.Dependencies{Rows: timetracking.TimeTrackingRows(capability), Ports: ports})
	if err != nil {
		return nil, err
	}
	return &WorkSessionSide{WorkSessionSide: timetracking.WorkSessionSide{AutoCheckIn: timetracking.AutoCheckInCapability(clock.Sessions()), Clock: clock, Staff: timetracking.StaffScheduleBindingLookup(staff)}}, nil
}

// TimeTrackingCleanupDependencies wire the retention cleanup the nightly
// sweep and the cleanup CLI run.
type TimeTrackingCleanupDependencies struct {
	Workforce workforce.Capability
	// Absences serves the staff absence rows with their retained relations.
	Absences AbsenceRecords
	// Audit records the deletion evidence; nil runs without it.
	Audit    DeletionAudit
	Settings TimeTrackingCleanupSettings
	Logger   *slog.Logger
}

// NewTimeTrackingCleanup builds the time-tracking retention cleanup over the
// Workforce work-session rows.
func NewTimeTrackingCleanup(deps TimeTrackingCleanupDependencies) (TimeTrackingCleanupService, error) {
	if deps.Workforce == nil {
		return nil, errors.New("time tracking cleanup: the Workforce capability is required")
	}
	return timetracking.NewCleanup(timetracking.CleanupDependencies{
		Sessions: deps.Workforce, Absences: deps.Absences, Audit: deps.Audit, Settings: deps.Settings, Logger: deps.Logger,
	})
}

type WorkSessionSide struct {
	timetracking.WorkSessionSide
	PayrollDispatch workforce.PayrollDispatch // #4805, composed by the root
}
type TimeTracking timetracking.TimeTracking
type AutoCheckIn interface {
	timetracking.AutoCheckInBinding
}
type TimeTrackingJobs interface {
	timetracking.TimeTrackingJobs
}
type StaffDayStatus interface {
	timetracking.StaffDayStatus
}
type StaffScheduleBindingLookup func(context.Context, int64) (StaffScheduleBinding, error)

func (s *WorkSessionSide) TimeTracking() (*TimeTracking, error) {
	value, err := s.WorkSessionSide.TimeTracking()
	return (*TimeTracking)(value), err
}

// NewEmploymentReset binds invitation resets to the ordinary audited readers.
func NewEmploymentReset(admin workforce.StaffRecordAdmin, capability workforce.Capability, employment workforce.StaffEmployments, audit workforce.StaffAdminAudit, ports TimeTrackingPorts) workforce.EmploymentResetter {
	return application.NewEmploymentReset(admin, capability, employment, audit, timetracking.NewEmploymentTimeReset(timetracking.Dependencies{Rows: timetracking.TimeTrackingRows(capability), Ports: ports}, application.NewEmploymentScheduleReset(capability, employment, audit)))
}
