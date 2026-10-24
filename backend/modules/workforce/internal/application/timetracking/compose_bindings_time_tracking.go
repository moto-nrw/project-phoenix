package timetracking

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// AutoCheckIn opens today's block when a staff member's first NFC scan or
// web stamp of the day arrives without one; Student Presence calls it.
type AutoCheckInBinding interface {
	EnsureCheckedIn(ctx context.Context, staffID int64, source string) (*workforce.WorkSession, error)
}

// StaffScheduleBindingLookup resolves the staff record a schedule update is
// written for: its current work time model and rotation anchor. The root
// binds it to the People Directory's staff read.
type StaffScheduleBindingLookup func(ctx context.Context, staffID int64) (StaffScheduleBinding, error)

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
	AutoCheckIn AutoCheckInBinding
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

// WorkSessionSide is the time tracking before the absence lifecycle; the root
// completes it with TimeTracking and binds the sick-report writes into the
// shift-plan-sync workflow (#1843, #4825).
type WorkSessionSide struct {
	AutoCheckIn AutoCheckInBinding
	Clock       *Clock
	Staff       StaffScheduleBindingLookup
}

// TimeTracking adds the absences and returns the complete time tracking.
func (s *WorkSessionSide) TimeTracking() (*TimeTracking, error) {
	if s.Clock == nil {
		return nil, errors.New("time tracking: the clock is required")
	}
	services := s.Clock.WithAbsences()
	return &TimeTracking{
		WorkSessions:       WorkSessionCapability(services.Sessions, s.Staff),
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
		AutoCheckIn:        s.AutoCheckIn,
		Jobs:               services.Sessions,
		StaffDay:           StaffDay{WorkSessionService: services.Sessions, StaffAbsenceService: services.Absences},
	}, nil
}

// staffDay serves today's staff status from the session and absence
// services, which already answer it.
type StaffDay struct {
	WorkSessionService
	StaffAbsenceService
}

func TimeTrackingRows(capability workforce.Capability) Rows {
	return Rows{
		TargetOverrides:  capability,
		MonthSnapshots:   MonthSnapshotCapability(capability),
		AbsenceTypes:     AbsenceTypes(capability),
		Sessions:         capability,
		Breaks:           capability,
		Adjustments:      capability,
		VacationQuotas:   capability,
		VacationOpenings: capability,
	}
}
