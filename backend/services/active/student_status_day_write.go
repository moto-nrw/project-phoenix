package active

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ErrStudentStatusDayReassigned signals that a student left the caller's scope
// between the initial authorization and the locked re-read inside the write
// transaction. Handlers map it to 403.
var ErrStudentStatusDayReassigned = errors.New("student reassigned out of caller scope")

// ErrStudentStatusDayPartialAbsenceConflict prevents broad day statuses from
// erasing a more precise time-specific excusal without an explicit decision.
var ErrStudentStatusDayPartialAbsenceConflict = errors.New("student status day conflicts with a partial absence")

// MaxStudentStatusDayConflictDetails is the maximum number of conflict rows
// returned in a 409 response body. Larger totals are reported via Total only so
// bulk class-trip writes cannot serialize tens of thousands of rows.
const MaxStudentStatusDayConflictDetails = 32

// StudentStatusDayConflictError reports active rows that prevented an atomic
// planned-status write. Callers must surface these rows to the user instead of
// clearing or replacing them. Conflicts may be a capped sample; Total is the
// full count when set (or len(Conflicts) when zero).
type StudentStatusDayConflictError struct {
	Conflicts []*activeModels.StudentStatusDay
	// Total is the full conflict count before sampling. Zero means "use
	// len(Conflicts)" for single-student writes that return the full set.
	Total int
}

func (e *StudentStatusDayConflictError) Error() string {
	return "student status days conflict with active rows"
}

// ConflictTotal returns the full conflict count for API payloads.
func (e *StudentStatusDayConflictError) ConflictTotal() int {
	if e == nil {
		return 0
	}
	if e.Total > 0 {
		return e.Total
	}
	return len(e.Conflicts)
}

// SampleConflicts returns at most MaxStudentStatusDayConflictDetails rows for
// a 409 body. Bulk writers should already keep Conflicts within this bound.
func (e *StudentStatusDayConflictError) SampleConflicts() []*activeModels.StudentStatusDay {
	if e == nil {
		return nil
	}
	if len(e.Conflicts) <= MaxStudentStatusDayConflictDetails {
		return e.Conflicts
	}
	return e.Conflicts[:MaxStudentStatusDayConflictDetails]
}

// StatusDayWriteContext carries the collaborators the status-day write
// orchestration needs but the StudentStatusDayService is not constructed with,
// keeping the repo-only constructor stable. The authorize callback keeps the
// JWT-permission decision at the HTTP boundary while the transaction
// composition lives in the service. Durable notifications run inside the
// transaction; after-commit runs only the ephemeral SSE fan-out.
type StatusDayWriteContext struct {
	DB             *bun.DB
	TenantID       int64
	StudentService users.StudentService
	Authorize      func(ctx context.Context, student *userModels.Student) bool
	AfterCommit    func(studentID int64)
	// AfterCreate receives the students whose current-day status newly became a
	// reportable absence inside the write transaction. It is separate from
	// AfterCommit so bulk writes can emit one aggregate absence notification
	// while retaining the per-student SSE fan-out.
	AfterCreate func(context.Context, []int64) error
}

// CreateForDates records the reported status for one student across the given
// dates inside a tenant transaction: locked-row re-authorization, rejecting
// active conflicts without partial writes, upserting the reported rows,
// mutating today's live sick/excused flags, and registering the after-commit
// broadcast.
func (s *StudentStatusDayService) CreateForDates(ctx context.Context, wc StatusDayWriteContext, studentID int64, status, reason string, dates []timezone.Date) error {
	if len(dates) == 0 {
		return errors.New("student status day dates are required")
	}
	now := s.now()
	today := timezone.DateFromTime(now)
	notePtr := strutil.TrimPtrToNil(&reason)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ bun.Tx) error {
		fresh, err := wc.StudentService.GetByIDForUpdate(ctx, studentID)
		if err != nil {
			return err
		}
		if !wc.Authorize(ctx, fresh) {
			return ErrStudentStatusDayReassigned
		}
		if err := lockStudentStatusDates(ctx, wc.DB, studentID, dates); err != nil {
			return err
		}
		if err := s.ensureNoPartialAbsenceConflicts(ctx, studentID, dates); err != nil {
			return err
		}

		conflicts, err := s.findRequestedActiveConflicts(ctx, studentID, dates)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return &StudentStatusDayConflictError{Conflicts: conflicts}
		}

		notifyAbsence, err := s.writeStatusForStudent(ctx, wc, fresh, status, dates, notePtr, now, today)
		if err != nil {
			return err
		}
		tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
		if notifyAbsence && wc.AfterCreate != nil {
			if err := wc.AfterCreate(ctx, []int64{studentID}); err != nil {
				return err
			}
		}
		return nil
	})
}

// BulkCreateForDates applies CreateForDates' orchestration to several students
// within a single tenant transaction. Authorization and conflict preflight run
// for the full set before any writes so a mixed-scope or conflicting selection
// cannot partially commit or overwrite existing status rows.
func (s *StudentStatusDayService) BulkCreateForDates(ctx context.Context, wc StatusDayWriteContext, studentIDs []int64, status, reason string, dates []timezone.Date) error {
	if len(dates) == 0 {
		return errors.New("student status day dates are required")
	}
	studentIDs = dedupeStudentIDs(studentIDs)
	now := s.now()
	today := timezone.DateFromTime(now)
	notePtr := strutil.TrimPtrToNil(&reason)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ bun.Tx) error {
		// Phase 1: lock and authorize every student before writing any row.
		// Order by ID for stable lock acquisition across concurrent bulk ops.
		sortedIDs := append([]int64(nil), studentIDs...)
		slices.Sort(sortedIDs)
		lockedStudents := make(map[int64]*userModels.Student, len(sortedIDs))
		for _, studentID := range sortedIDs {
			fresh, err := wc.StudentService.GetByIDForUpdate(ctx, studentID)
			if err != nil {
				return err
			}
			if !wc.Authorize(ctx, fresh) {
				return ErrStudentStatusDayReassigned
			}
			lockedStudents[studentID] = fresh
		}
		for _, studentID := range sortedIDs {
			if err := lockStudentStatusDates(ctx, wc.DB, studentID, dates); err != nil {
				return err
			}
			if err := s.ensureNoPartialAbsenceConflicts(ctx, studentID, dates); err != nil {
				return err
			}
		}

		// Phase 2: preflight every student/date pair so no existing sick,
		// excused, or class-trip row is cleared or overwritten. Keep only a
		// sample of rows for the 409 body; Total carries the full count.
		conflictSamples := make([]*activeModels.StudentStatusDay, 0, MaxStudentStatusDayConflictDetails)
		conflictTotal := 0
		for _, studentID := range sortedIDs {
			studentConflicts, err := s.findRequestedActiveConflicts(ctx, studentID, dates)
			if err != nil {
				return err
			}
			conflictTotal += len(studentConflicts)
			remaining := MaxStudentStatusDayConflictDetails - len(conflictSamples)
			if remaining <= 0 {
				continue
			}
			if len(studentConflicts) > remaining {
				conflictSamples = append(conflictSamples, studentConflicts[:remaining]...)
			} else {
				conflictSamples = append(conflictSamples, studentConflicts...)
			}
		}
		if conflictTotal > 0 {
			return &StudentStatusDayConflictError{
				Conflicts: conflictSamples,
				Total:     conflictTotal,
			}
		}

		// Phase 3: write only after the full selection is in scope and clear.
		absenceStudentIDs := make([]int64, 0, len(studentIDs))
		for _, studentID := range studentIDs {
			notifyAbsence, err := s.writeStatusForStudent(ctx, wc, lockedStudents[studentID], status, dates, notePtr, now, today)
			if err != nil {
				return err
			}
			if notifyAbsence {
				absenceStudentIDs = append(absenceStudentIDs, studentID)
			}
			studentID := studentID
			tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
		}
		if len(absenceStudentIDs) > 0 && wc.AfterCreate != nil {
			if err := wc.AfterCreate(ctx, absenceStudentIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

func lockStudentStatusDates(ctx context.Context, db *bun.DB, studentID int64, dates []timezone.Date) error {
	sortedDates := append([]timezone.Date(nil), dates...)
	slices.SortFunc(sortedDates, timezone.Date.Compare)
	for _, date := range sortedDates {
		if err := careplanning.LockExceptionDay(ctx, db, studentID, date.String()); err != nil {
			return err
		}
	}
	return nil
}

func (s *StudentStatusDayService) ensureNoPartialAbsenceConflicts(
	ctx context.Context, studentID int64, dates []timezone.Date,
) error {
	if s.pickupExceptions == nil || len(dates) == 0 {
		return nil
	}
	requested := make(map[scheduleModels.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[scheduleModels.Date(date)] = struct{}{}
	}
	rows, err := s.pickupExceptions.FindByStudentIDAndDateRange(
		ctx,
		studentID,
		scheduleModels.Date(slices.MinFunc(dates, timezone.Date.Compare)),
		scheduleModels.Date(slices.MaxFunc(dates, timezone.Date.Compare)),
	)
	if err != nil {
		return err
	}
	for _, row := range rows {
		// Auto-derived excusals (pulled-forward pickup time, #2360) do not
		// block a broad day status: the two coexist via disjoint slot
		// ownership, and refusing would make every pickup change block a sick
		// report. Only a staff-set manual partial absence conflicts.
		if row != nil && row.HasManualPartialAbsence() {
			if _, matches := requested[row.ExceptionDate]; matches {
				return ErrStudentStatusDayPartialAbsenceConflict
			}
		}
	}
	return nil
}

// findRequestedActiveConflicts returns active status-day rows that already
// cover any of the requested dates for one student. Callers must reject the
// whole write when this list is non-empty.
func (s *StudentStatusDayService) findRequestedActiveConflicts(ctx context.Context, studentID int64, dates []timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	requestedDates := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		requestedDates[date] = struct{}{}
	}
	activeRows, err := s.repo.FindActiveByStudentAndDateRange(
		ctx,
		studentID,
		slices.MinFunc(dates, timezone.Date.Compare),
		slices.MaxFunc(dates, timezone.Date.Compare),
	)
	if err != nil {
		return nil, err
	}
	conflicts := make([]*activeModels.StudentStatusDay, 0, len(activeRows))
	for _, row := range activeRows {
		if _, requested := requestedDates[row.Date]; requested {
			conflicts = append(conflicts, row)
		}
	}
	return conflicts, nil
}

func dedupeStudentIDs(studentIDs []int64) []int64 {
	seen := make(map[int64]struct{}, len(studentIDs))
	unique := make([]int64, 0, len(studentIDs))
	for _, studentID := range studentIDs {
		if _, exists := seen[studentID]; exists {
			continue
		}
		seen[studentID] = struct{}{}
		unique = append(unique, studentID)
	}
	return unique
}

// DeleteByID clears a single status-day row after ownership and locked-row
// re-authorization checks, resetting today's live flags when the row is today.
func (s *StudentStatusDayService) DeleteByID(ctx context.Context, wc StatusDayWriteContext, statusDayID, studentID int64) error {
	now := s.now()
	today := timezone.DateFromTime(now)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ bun.Tx) error {
		row, err := s.repo.FindActiveByID(ctx, statusDayID)
		if err != nil {
			return err
		}
		if row.StudentID != studentID {
			return sql.ErrNoRows
		}

		fresh, err := wc.StudentService.GetByIDForUpdate(ctx, studentID)
		if err != nil {
			return err
		}
		if !wc.Authorize(ctx, fresh) {
			return ErrStudentStatusDayReassigned
		}

		if err := s.repo.MarkClearedByID(ctx, row.ID, now, activeModels.StudentStatusSourceManual); err != nil {
			return err
		}
		if row.Date == today {
			ClearLiveStatusForToday(fresh, row.Status)
			if err := wc.StudentService.Update(ctx, fresh); err != nil {
				return err
			}
		}

		tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
		return nil
	})
}

func (s *StudentStatusDayService) writeStatusForStudent(ctx context.Context, wc StatusDayWriteContext, fresh *userModels.Student, status string, dates []timezone.Date, notePtr *string, now time.Time, today timezone.Date) (bool, error) {
	notifyAbsence := isNewReportableAbsence(fresh, status, dates, today)
	if err := s.clearOtherStatusDaysForDates(ctx, fresh.ID, status, dates, now); err != nil {
		return false, err
	}
	for _, date := range dates {
		if err := s.repo.UpsertReported(ctx, &activeModels.StudentStatusDay{
			StudentID:  fresh.ID,
			Date:       date,
			Status:     status,
			ReportedAt: now,
			Source:     activeModels.StudentStatusSourcePlanned,
			Note:       notePtr,
		}); err != nil {
			return false, err
		}
	}
	if slices.Contains(dates, today) {
		ApplyLiveStatusForToday(fresh, status, now)
		if err := wc.StudentService.Update(ctx, fresh); err != nil {
			return false, err
		}
	}
	return notifyAbsence, nil
}

func isNewReportableAbsence(student *userModels.Student, status string, dates []timezone.Date, today timezone.Date) bool {
	if !slices.Contains(dates, today) {
		return false
	}
	switch status {
	case activeModels.StudentStatusDaySick:
		return student.Sick == nil || !*student.Sick
	case activeModels.StudentStatusDayExcused:
		return student.Excused == nil || !*student.Excused
	default:
		return false
	}
}

func (s *StudentStatusDayService) clearOtherStatusDaysForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, now time.Time) error {
	for _, otherStatus := range activeModels.StudentStatusDayStatusesExcept(status) {
		if err := s.repo.MarkClearedForDates(ctx, studentID, otherStatus, dates, now, activeModels.StudentStatusSourceManual); err != nil {
			return err
		}
	}
	return nil
}

// ApplyLiveStatusForToday mutates a student's live sick/excused flags to reflect
// a status reported for today (mutually exclusive across sick/excused/class
// trip).
func ApplyLiveStatusForToday(student *userModels.Student, status string, now time.Time) {
	trueVal := true
	falseVal := false
	switch status {
	case activeModels.StudentStatusDaySick:
		student.Sick = &trueVal
		student.SickSince = &now
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case activeModels.StudentStatusDayExcused:
		student.Excused = &trueVal
		student.ExcusedSince = &now
		student.Sick = &falseVal
		student.SickSince = nil
	case activeModels.StudentStatusDayClassTrip:
		student.Sick = &falseVal
		student.SickSince = nil
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
}

// ClearLiveStatusForToday resets the live flags a today status set.
func ClearLiveStatusForToday(student *userModels.Student, status string) {
	falseVal := false
	switch status {
	case activeModels.StudentStatusDaySick:
		student.Sick = &falseVal
		student.SickSince = nil
	case activeModels.StudentStatusDayExcused:
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case activeModels.StudentStatusDayClassTrip:
		student.Sick = &falseVal
		student.SickSince = nil
		student.Excused = &falseVal
		student.ExcusedSince = nil
	}
}
