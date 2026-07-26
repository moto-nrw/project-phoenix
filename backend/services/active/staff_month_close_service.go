package active

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// Sentinel errors for handler mapping (#1417).
var (
	// ErrMonthCloseInvalid marks a caller mistake (bad month, missing
	// reason) — HTTP 400.
	ErrMonthCloseInvalid = errors.New("invalid month close request")
	// ErrMonthNotClosable marks a month that may not be frozen yet: still
	// running, or entirely before the account start — HTTP 400.
	ErrMonthNotClosable = errors.New("month cannot be closed")
	// ErrMonthNotClosed marks a reopen of a month that is not frozen — HTTP 404.
	ErrMonthNotClosed = errors.New("month is not closed")
	// ErrLaterMonthClosed prevents changing the snapshot chain behind a later
	// frozen month — HTTP 409.
	ErrLaterMonthClosed = errors.New("a later month is still closed")
)

// maxSnapshotYear is the upper bound validateMonth accepts, used to ask the
// repository for "the newest snapshot at all" without a second query shape.
const maxSnapshotYear = 2100

// monthCloseStaffLister is implemented by users.StaffRepository. Narrow on
// purpose: the close service needs the tenant's staff members, not the staff
// domain service (which would pull in a service-to-service dependency).
type monthCloseStaffLister interface {
	ListAllWithPerson(ctx context.Context) ([]*userModels.Staff, error)
}

// MonthCloseResult reports what a school-wide close did.
type MonthCloseResult struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	// ClosedStaff counts newly written snapshots, SkippedStaff the ones that
	// were already frozen (a second click is idempotent, not an error).
	ClosedStaff  int                                       `json:"closed_staff"`
	SkippedStaff int                                       `json:"skipped_staff"`
	Snapshots    []*activeModels.StaffMonthBalanceSnapshot `json:"snapshots"`
}

// StaffMonthCloseService owns the Monatsabschluss (#1417): freezing a month's
// closing balance so a retroactive shift, session, or absence correction can no
// longer rewrite the Übertrag of every later month.
//
// It is a separate service from StaffBalanceAdjustmentService on purpose. The
// ledger owns dated bookings with a signed delta; this owns a per-month
// freeze marker with no delta at all, plus a school-wide loop. Fusing the two
// would put two unrelated invariant sets in one file.
type StaffMonthCloseService interface {
	// CloseMonth freezes the month for every staff member of the tenant. It
	// must run inside the ambient tenant transaction: the advisory locks are
	// transaction-scoped, and one failing staff member has to roll the whole
	// close back (a partially closed month has no defined carry semantics).
	CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (*MonthCloseResult, error)
	// ReopenMonth supersedes one staff member's snapshot so the chain
	// recomputes. Per staff, not school-wide: a legitimate correction normally
	// concerns one person.
	ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) error
	// ListMonthStatus returns the active snapshots of one month.
	ListMonthStatus(ctx context.Context, year, month int) ([]*activeModels.StaffMonthBalanceSnapshot, error)
}

type staffMonthCloseService struct {
	snapshotRepo activeModels.StaffMonthBalanceSnapshotRepository
	monthService WorkTimeMonthService
	staffLister  monthCloseStaffLister
	settings     monthSettingsResolver
	broadcaster  realtime.Broadcaster
	logger       *slog.Logger
}

func NewStaffMonthCloseService(
	snapshotRepo activeModels.StaffMonthBalanceSnapshotRepository,
	monthService WorkTimeMonthService,
	staffLister monthCloseStaffLister,
	settings monthSettingsResolver,
	logger *slog.Logger,
) StaffMonthCloseService {
	return &staffMonthCloseService{
		snapshotRepo: snapshotRepo,
		monthService: monthService,
		staffLister:  staffLister,
		settings:     settings,
		logger:       logger,
	}
}

func (s *staffMonthCloseService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

// SetBroadcaster injects the tenant-wide SSE broadcaster. It stays outside
// StaffMonthCloseService so existing API-layer mocks stay unchanged.
func (s *staffMonthCloseService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

func (s *staffMonthCloseService) broadcastTimeTrackingChanged(ctx context.Context) {
	queueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
}

func calendarMonthHasEnded(key monthKey, today timezone.Date) bool {
	return key.lastDay().Before(today)
}

// validateCloseRequest checks the month, the reason, and whether the month may
// be frozen at all.
//
// The "month is over" check is the load-bearing one. The month math clamps
// Soll at the clamp date, and the close deliberately clamps at the month's
// last day to get a true closing value. For a month that is still running,
// the full month's Soll would be charged against an Ist that stops now. This
// also applies on the last calendar day: that day's remaining work has not
// happened yet. The frozen value would carry the artificial deficit forward
// forever.
// Closing with the clamp at today is not a fix; the result would not be the
// month's closing balance and the splice for the next month would be wrong in
// the other direction.
func (s *staffMonthCloseService) validateCloseRequest(ctx context.Context, year, month int, reason string) (monthKey, string, error) {
	if err := validateMonth(year, month); err != nil {
		return monthKey{}, "", fmt.Errorf("%w: %s", ErrMonthCloseInvalid, err.Error())
	}
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return monthKey{}, "", fmt.Errorf("%w: a reason is required", ErrMonthCloseInvalid)
	}
	key := monthKey{Year: year, Month: month}
	if !calendarMonthHasEnded(key, timezone.TodayDate()) {
		return monthKey{}, "", fmt.Errorf(
			"%w: %04d-%02d is not over yet",
			ErrMonthNotClosable, year, month,
		)
	}
	anchor, err := resolveAccountAnchor(ctx, s.settings, s.getLogger(), key)
	if err != nil {
		return monthKey{}, "", fmt.Errorf("failed to resolve account start for month close: %w", err)
	}
	// A month wholly before the account start never enters a carry chain, so
	// its snapshot could not seed one either. The read side ignores such rows;
	// this is the write-side half of that fence.
	if key.lastDay().Before(anchor) {
		return monthKey{}, "", fmt.Errorf(
			"%w: %04d-%02d lies before the account start %s",
			ErrMonthNotClosable, year, month, anchor.String(),
		)
	}
	return key, trimmed, nil
}

func (s *staffMonthCloseService) CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (*MonthCloseResult, error) {
	if closedBy <= 0 {
		return nil, fmt.Errorf("%w: closed_by is required", ErrMonthCloseInvalid)
	}
	key, trimmedReason, err := s.validateCloseRequest(ctx, year, month, reason)
	if err != nil {
		return nil, err
	}

	staffMembers, err := s.staffLister.ListAllWithPerson(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list staff for month close: %w", err)
	}
	// Ascending staff ID: every other holder of the per-staff advisory lock
	// takes exactly one, so a consistent order here rules out deadlocks
	// against a concurrent payout or schedule edit.
	slices.SortFunc(staffMembers, func(a, b *userModels.Staff) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})

	result := &MonthCloseResult{Year: year, Month: month}
	for _, staff := range staffMembers {
		snapshot, alreadyClosed, err := s.closeStaffMonth(ctx, staff.ID, closedBy, key, trimmedReason)
		if err != nil {
			return nil, err
		}
		if alreadyClosed {
			result.SkippedStaff++
			continue
		}
		result.ClosedStaff++
		result.Snapshots = append(result.Snapshots, snapshot)
	}

	s.getLogger().Info("month balance closed",
		"year", year,
		"month", month,
		"closed_by", closedBy,
		"closed_staff", result.ClosedStaff,
		"skipped_staff", result.SkippedStaff,
	)
	if result.ClosedStaff > 0 {
		s.broadcastTimeTrackingChanged(ctx)
	}
	return result, nil
}

// closeStaffMonth freezes one staff member's month. Staff without a schedule
// or any recorded time get a zero row on purpose: "closed" has to be a
// school-wide fact, otherwise a later card for that person would find no
// snapshot and silently fall back to the full chain.
func (s *staffMonthCloseService) closeStaffMonth(
	ctx context.Context,
	staffID, closedBy int64,
	key monthKey,
	reason string,
) (*activeModels.StaffMonthBalanceSnapshot, bool, error) {
	if err := s.snapshotRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, false, fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	existing, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, key.Year, key.Month)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check existing month snapshot for staff %d: %w", staffID, err)
	}
	if existing != nil && existing.Year == key.Year && existing.Month == key.Month {
		return nil, true, nil
	}
	latest, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, maxSnapshotYear, 12)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check later month snapshots for staff %d: %w", staffID, err)
	}
	if latest != nil && key.before(monthKey{Year: latest.Year, Month: latest.Month}) {
		return nil, false, fmt.Errorf(
			"%w: %04d-%02d must be reopened first",
			ErrLaterMonthClosed, latest.Year, latest.Month,
		)
	}
	summary, err := s.monthService.GetMonthSummaryAtMonthEnd(ctx, staffID, key.Year, key.Month)
	if err != nil {
		return nil, false, fmt.Errorf("failed to compute month summary for staff %d: %w", staffID, err)
	}
	snapshot := &activeModels.StaffMonthBalanceSnapshot{
		StaffID:               staffID,
		Year:                  key.Year,
		Month:                 key.Month,
		ClosingBalanceMinutes: summary.ClosingBalanceMinutes,
		CarryInMinutes:        summary.CarryInMinutes,
		// The month is over, so TargetMinutesToDate equals TargetMinutes; use
		// the clamped one to keep target and actual on the same footing.
		TargetMinutes:     summary.TargetMinutesToDate,
		ActualMinutes:     summary.ActualMinutes,
		CreditedMinutes:   summary.CreditedSickMinutes + summary.CreditedVacationMinutes + summary.CreditedOtherMinutes,
		AdjustmentMinutes: summary.AdjustmentMinutes,
		ClosedAt:          time.Now(),
		ClosedBy:          closedBy,
		CloseReason:       reason,
		Source:            activeModels.SnapshotSourceAdmin,
	}
	if err := s.snapshotRepo.Create(ctx, snapshot); err != nil {
		return nil, false, fmt.Errorf("failed to create month snapshot for staff %d: %w", staffID, err)
	}
	s.getLogger().Debug("month balance snapshot written",
		"staff_id", staffID,
		"year", key.Year,
		"month", key.Month,
		"closing_balance_minutes", snapshot.ClosingBalanceMinutes,
	)
	return snapshot, false, nil
}

func (s *staffMonthCloseService) ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) error {
	if staffID <= 0 || reopenedBy <= 0 {
		return fmt.Errorf("%w: staff id and reopened_by are required", ErrMonthCloseInvalid)
	}
	if err := validateMonth(year, month); err != nil {
		return fmt.Errorf("%w: %s", ErrMonthCloseInvalid, err.Error())
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return fmt.Errorf("%w: a reason is required", ErrMonthCloseInvalid)
	}
	if err := s.snapshotRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff balance writes: %w", err)
	}

	snapshot, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, year, month)
	if err != nil {
		return fmt.Errorf("failed to load month snapshot: %w", err)
	}
	if snapshot == nil || snapshot.Year != year || snapshot.Month != month {
		return fmt.Errorf("%w: %04d-%02d for staff %d", ErrMonthNotClosed, year, month, staffID)
	}

	// Reopen must proceed newest-first. Reopening this month while a later one
	// stays frozen would change that later month's drift while everything
	// after it remains pinned to a value derived from the old history — a
	// state no admin can reason about.
	latest, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, maxSnapshotYear, 12)
	if err != nil {
		return fmt.Errorf("failed to check later month snapshots: %w", err)
	}
	if latest != nil && (latest.Year != year || latest.Month != month) {
		return fmt.Errorf(
			"%w: %04d-%02d must be reopened first",
			ErrLaterMonthClosed, latest.Year, latest.Month,
		)
	}

	now := time.Now()
	snapshot.ReopenedAt = &now
	snapshot.ReopenedBy = &reopenedBy
	snapshot.ReopenReason = trimmedReason
	if _, err := s.snapshotRepo.UpdateColumns(ctx, snapshot, "reopened_at", "reopened_by", "reopen_reason"); err != nil {
		return fmt.Errorf("failed to reopen month snapshot: %w", err)
	}

	s.getLogger().Info("month balance reopened",
		"staff_id", staffID,
		"year", year,
		"month", month,
		"reopened_by", reopenedBy,
		"frozen_closing_balance_minutes", snapshot.ClosingBalanceMinutes,
	)
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffMonthCloseService) ListMonthStatus(ctx context.Context, year, month int) ([]*activeModels.StaffMonthBalanceSnapshot, error) {
	if err := validateMonth(year, month); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrMonthCloseInvalid, err.Error())
	}
	return s.snapshotRepo.GetByMonth(ctx, year, month)
}
