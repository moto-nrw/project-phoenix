package active

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ErrStudentStatusDayReassigned signals that a student left the caller's scope
// between the initial authorization and the locked re-read inside the write
// transaction. Handlers map it to 403.
var ErrStudentStatusDayReassigned = errors.New("student reassigned out of caller scope")

// StatusDayWriteContext carries the collaborators the status-day write
// orchestration needs but the StudentStatusDayService is not constructed with,
// keeping the repo-only constructor stable. The authorize callback keeps the
// JWT-permission decision at the HTTP boundary while the transaction
// composition lives in the service; after-commit runs the SSE fan-out.
type StatusDayWriteContext struct {
	DB             *bun.DB
	TenantID       int64
	StudentService users.StudentService
	Authorize      func(ctx context.Context, student *userModels.Student) bool
	AfterCommit    func(studentID int64)
}

// CreateForDates records the reported status for one student across the given
// dates inside a tenant transaction: locked-row re-authorization, clearing the
// conflicting statuses, upserting the reported rows, mutating today's live
// sick/excused flags, and registering the after-commit broadcast.
func (s *StudentStatusDayService) CreateForDates(ctx context.Context, wc StatusDayWriteContext, studentID int64, status, reason string, dates []timezone.Date) error {
	now := time.Now()
	today := timezone.TodayDate()
	notePtr := strutil.TrimPtrToNil(&reason)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.writeStatusForStudent(ctx, wc, studentID, status, dates, notePtr, now, today); err != nil {
			return err
		}
		tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
		return nil
	})
}

// BulkCreateForDates applies CreateForDates' orchestration to several students
// within a single tenant transaction.
func (s *StudentStatusDayService) BulkCreateForDates(ctx context.Context, wc StatusDayWriteContext, studentIDs []int64, status, reason string, dates []timezone.Date) error {
	now := time.Now()
	today := timezone.TodayDate()
	notePtr := strutil.TrimPtrToNil(&reason)
	return tenant.WithTenantTx(ctx, wc.DB, wc.TenantID, func(ctx context.Context, _ bun.Tx) error {
		for _, studentID := range studentIDs {
			if err := s.writeStatusForStudent(ctx, wc, studentID, status, dates, notePtr, now, today); err != nil {
				return err
			}
			studentID := studentID
			tenant.RegisterAfterCommit(ctx, func() { wc.AfterCommit(studentID) })
		}
		return nil
	})
}

// DeleteByID clears a single status-day row after ownership and locked-row
// re-authorization checks, resetting today's live flags when the row is today.
func (s *StudentStatusDayService) DeleteByID(ctx context.Context, wc StatusDayWriteContext, statusDayID, studentID int64) error {
	now := time.Now()
	today := timezone.TodayDate()
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

func (s *StudentStatusDayService) writeStatusForStudent(ctx context.Context, wc StatusDayWriteContext, studentID int64, status string, dates []timezone.Date, notePtr *string, now time.Time, today timezone.Date) error {
	fresh, err := wc.StudentService.GetByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}
	if !wc.Authorize(ctx, fresh) {
		return ErrStudentStatusDayReassigned
	}

	if err := s.clearOtherStatusDaysForDates(ctx, fresh.ID, status, dates, now); err != nil {
		return err
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
			return err
		}
	}
	if slices.Contains(dates, today) {
		ApplyLiveStatusForToday(fresh, status, now)
		if err := wc.StudentService.Update(ctx, fresh); err != nil {
			return err
		}
	}
	return nil
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
