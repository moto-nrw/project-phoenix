package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Error message constants to avoid duplication
const (
	errNoActiveSession             = "no active session found"
	errGetCurrentSession           = "failed to get current session: %w"
	errInvalidSessionData          = "invalid session data: %w"
	maxPlannedBreakDurationMinutes = 240
)

// workSessionService implements WorkSessionService
type workSessionService struct {
	repo        WorkSessionRecords
	breakRepo   WorkSessionBreakRecords
	auditRepo   WorkSessionAudit
	absenceRepo AbsenceRecords
	// absenceTypes resolves school-defined Abwesenheitsarten for exports
	// (#2403). Setter injection; nil in bare-constructed unit fixtures.
	absenceTypes   AbsenceTypeReader
	supervisorRepo GroupSupervisorRepository
	groupRepo      GroupRepository
	db             DatabaseHandle
	staffRepo      WorkSessionStaff
	scheduleRepo   WorkSessionSchedules
	workModelRepo  WorkSessionTimeModels
	settings       settingsResolver
	staffShiftRepo WorkSessionShifts
	holidayReader  HolidayDatesReader
	// weeklyOverrides re-prices Sonderarbeitszeit weeks; nil keeps them.
	weeklyOverrides WeeklyTargetOverrides
	broadcaster     EventPublisher
	logger          *slog.Logger
	nowFunc         func() time.Time
	renderPDF       TimeTrackingPDFRenderer
	renderWorkbook  TimeTrackingWorkbookRenderer
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (s *workSessionService) getLogger() *slog.Logger {
	return loggerOrDefault(s.logger)
}

func (s *workSessionService) broadcastTimeTrackingChanged(ctx context.Context) {
	queueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
}

func (s *workSessionService) lockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	if err := s.repo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	return nil
}

func (s *workSessionService) lockStaffBalanceWritesOrdered(ctx context.Context, staffIDs []int64) error {
	unique := make(map[int64]struct{}, len(staffIDs))
	for _, staffID := range staffIDs {
		unique[staffID] = struct{}{}
	}
	ordered := slices.Collect(maps.Keys(unique))
	slices.Sort(ordered)
	for _, staffID := range ordered {
		if err := s.lockStaffBalanceWrites(ctx, staffID); err != nil {
			return err
		}
	}
	return nil
}

// NewWorkSessionService creates a new work session service

func NewWorkSessionService(repo WorkSessionRecords, breakRepo WorkSessionBreakRecords, auditRepo WorkSessionAudit, absenceRepo AbsenceRecords, supervisorRepo GroupSupervisorRepository, groupRepo GroupRepository, staffRepo WorkSessionStaff, scheduleRepo WorkSessionSchedules, workModelRepo WorkSessionTimeModels, settings settingsResolver, logger *slog.Logger, db DatabaseHandle, renderPDF TimeTrackingPDFRenderer, renderWorkbook TimeTrackingWorkbookRenderer, opts ...WorkSessionOption) WorkSessionService {
	service := &workSessionService{repo: repo, breakRepo: breakRepo, auditRepo: auditRepo, absenceRepo: absenceRepo, supervisorRepo: supervisorRepo, groupRepo: groupRepo, staffRepo: staffRepo, scheduleRepo: scheduleRepo, workModelRepo: workModelRepo, settings: settings, logger: logger, db: db, renderPDF: renderPDF, renderWorkbook: renderWorkbook}
	for _, opt := range opts {
		opt(service)
	}
	if db != nil {
		return &workSessionCommands{service, service}
	}
	return service
}

func (s *workSessionService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

// listSessionsByStaffAndDate applies the ordinary repository filter path for
// the two equality conditions used by check-in and auto-check-in. Keeping the
// lookup here avoids a per-field repository method while retaining the
// chronological block order required by the caller.
func (s *workSessionService) listSessionsByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) ([]*WorkSession, error) {
	sessionsRecords, err := s.repo.ListWorkSessions(ctx, workforce.WorkSessionFilter{
		StaffID: staffID, Date: date.String(),
		Order: []workforce.WorkSessionOrder{{Field: workforce.WorkSessionOrderCheckInTime}},
	})
	sessions := workSessionsToRecords(sessionsRecords)
	err = rowReadError("list with options", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list work sessions by staff and date: %w", err)
	}
	return sessions, nil
}

type workSessionCommands struct {
	WorkSessionService
	commands *workSessionService
}
