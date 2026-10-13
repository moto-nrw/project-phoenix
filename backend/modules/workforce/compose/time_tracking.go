package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
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
	ShiftPlanSyncer             = timetracking.ShiftPlanSyncer
	StaffDisplayNameQuery       = timetracking.StaffDisplayNameQuery
	StaffScheduleAssignment     = timetracking.StaffScheduleAssignment
	StaffScheduleBinding        = timetracking.StaffScheduleBinding
	StaffScheduleQuery          = timetracking.StaffScheduleQuery
	TimeExportStaff             = timetracking.TimeExportStaff
	TimeExportStaffQuery        = timetracking.TimeExportStaffQuery
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

// AutoCheckIn opens today's block when a staff member's first NFC scan or
// web stamp of the day arrives without one; Student Presence calls it.
type AutoCheckIn interface {
	EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*timerecords.WorkSession, error)
}

// TimeTrackingJobs are the sweeps the scheduler runs per tenant: closing the
// sessions left open overnight, ending breaks past their planned end and the
// auto-checkout at the planned shift end (#1798).
type TimeTrackingJobs interface {
	CleanupOpenSessions(ctx context.Context) (int, error)
	AutoEndExpiredBreaks(ctx context.Context) (int, error)
	AutoCheckoutDueSessions(ctx context.Context, grace time.Duration) (int, error)
}

// TimeTracking is the composed time-tracking side of Workforce.
type TimeTracking struct {
	WorkSessions       workforce.WorkSessions
	TimeClock          workforce.TimeClock
	StaffAbsences      workforce.StaffAbsences
	WorkTimeMonths     workforce.WorkTimeMonths
	BalanceAdjustments workforce.BalanceAdjustments
	MonthClosing       workforce.MonthClosing
	Overview           workforce.StaffOverview
	AuditLog           workforce.TimeTrackingAuditLog
	TimeExport         workforce.StaffTimeExport
	VacationTakeovers  workforce.VacationTakeovers
	OpeningBalances    workforce.OpeningBalanceBookings
	// AutoCheckIn serves Student Presence's NFC auto-check-in.
	AutoCheckIn AutoCheckIn
	// Jobs are the scheduler's open-session, break and auto-checkout sweeps.
	Jobs TimeTrackingJobs
	// StaffDay is today's supervision, work and absence status the staff
	// list shows.
	StaffDay StaffDayStatus
}

// StaffDayStatus reads today's status of the staff members.
type StaffDayStatus interface {
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)
	GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error)
	GetTodayAbsenceLabelMap(ctx context.Context) (map[int64]string, error)
}

// NewTimeTracking composes the time-tracking services over the Workforce
// capability rows and the root's ports, and binds the public contracts; staff
// resolves the record a schedule update is written for.
func NewTimeTracking(capability workforce.Capability, ports TimeTrackingPorts, staff StaffScheduleBindingLookup) (*TimeTracking, error) {
	if capability == nil || staff == nil {
		return nil, errors.New("time tracking: the Workforce capability and the staff lookup are required")
	}
	services, err := timetracking.New(timetracking.Dependencies{Rows: timeTrackingRows(capability), Ports: ports})
	if err != nil {
		return nil, err
	}
	return &TimeTracking{
		WorkSessions:       WorkSessionCapability(services.Sessions, staff),
		TimeClock:          TimeClockCapability(services.Sessions),
		StaffAbsences:      StaffAbsenceCapability(services.Absences),
		WorkTimeMonths:     WorkTimeMonthCapability(services.Months),
		BalanceAdjustments: BalanceAdjustmentCapability(services.Ledger),
		MonthClosing:       MonthClosingCapability(services.Closing),
		Overview:           StaffOverviewCapability(services.Overview),
		AuditLog:           TimeTrackingAuditLogCapability(services.AuditLog),
		TimeExport:         StaffTimeExportCapability(services.Export),
		VacationTakeovers:  VacationTakeoverCapability(services.Absences),
		OpeningBalances:    OpeningBalanceBookingCapability(services.Ledger),
		AutoCheckIn:        services.Sessions,
		Jobs:               services.Sessions,
		StaffDay:           staffDay{WorkSessionService: services.Sessions, StaffAbsenceService: services.Absences},
	}, nil
}

// staffDay serves today's staff status from the session and absence
// services, which already answer it.
type staffDay struct {
	timetracking.WorkSessionService
	timetracking.StaffAbsenceService
}

func timeTrackingRows(capability workforce.Capability) timetracking.Rows {
	return timetracking.Rows{
		TargetOverrides:  capability,
		MonthSnapshots:   MonthSnapshotCapability(capability),
		AbsenceTypes:     AbsenceTypes(capability),
		Sessions:         newWorkSessionRows(capability),
		Breaks:           newWorkSessionBreakRows(capability),
		Adjustments:      newBalanceAdjustmentRows(capability),
		VacationQuotas:   newVacationQuotaRows(capability),
		VacationOpenings: newVacationOpeningRows(capability),
	}
}

// TimeTrackingCleanupDependencies wire the retention cleanup the nightly
// sweep and the cleanup CLI run.
type TimeTrackingCleanupDependencies struct {
	Workforce workforce.Capability
	// Absences serves the staff absence rows with their retained relations.
	Absences timerecords.StaffAbsenceRepository
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
		Sessions: newWorkSessionRows(deps.Workforce), Absences: deps.Absences, Audit: deps.Audit, Settings: deps.Settings, Logger: deps.Logger,
	})
}
