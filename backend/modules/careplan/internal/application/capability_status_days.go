package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type StatusDayStore interface {
	FindStudentStatusDay(context.Context, int64, bool) (careplan.StudentStatusDay, bool, domain.OperationStats, error)
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, domain.OperationStats, error)
	CountStudentStatusDays(context.Context, *careplan.StudentScheduleQueryOptions) (int, domain.OperationStats, error)
	CountEffectiveStudentAbsences(context.Context, careplan.Date) (careplan.StudentStatusCounts, domain.OperationStats, error)
	ListStatusDaySummaries(context.Context, careplan.Date, careplan.Date) ([]careplan.StatusDaySummary, domain.OperationStats, error)
	ExistingStudentStatusDayIDs(context.Context, []int64) ([]int64, domain.OperationStats, error)
	CountCarePlanDeletionRecords(context.Context, int64) (careplan.CarePlanDeletionCounts, domain.OperationStats, error)
	UpsertStudentStatusDay(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, domain.OperationStats, error)
	ClearStudentStatusDays(context.Context, int64, string, []careplan.Date, time.Time, string) (domain.OperationStats, error)
	ClearStudentStatusDayByID(context.Context, int64, time.Time, string) (domain.OperationStats, error)
	ArchiveStudentStatusFlags(context.Context, careplan.StatusFlagArchive) (int64, domain.OperationStats, error)
	DeleteStudentStatusDaysBefore(context.Context, careplan.Date) (int64, domain.OperationStats, error)
}

func (s *Service) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (careplan.StudentStatusDay, error) {
	return requestValue(ctx, s, "find_student_status_day", func(ctx context.Context) (careplan.StudentStatusDay, domain.OperationStats, error) {
		value, found, stats, err := s.statusQueries.FindStudentStatusDay(ctx, id, activeOnly)
		if err == nil && !found {
			err = careplan.ErrStudentStatusDayNotFound
		}
		return value, stats, err
	})
}

func (s *Service) ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StudentIDs = uniquePositive(filter.StudentIDs)

	return requestValue(ctx, s, "list_student_status_days", func(ctx context.Context) ([]careplan.StudentStatusDay, domain.OperationStats, error) {
		return s.statusQueries.ListStudentStatusDays(ctx, filter)
	})
}

func (s *Service) CountStudentStatusDays(ctx context.Context, options *careplan.StudentScheduleQueryOptions) (int, error) {
	return requestValue(ctx, s, "count_student_status_days", func(ctx context.Context) (int, domain.OperationStats, error) {
		return s.statusDays.CountStudentStatusDays(ctx, options)
	})
}

func (s *Service) CountEffectiveStudentAbsences(ctx context.Context, date careplan.Date) (careplan.StudentStatusCounts, error) {
	return requestValue(ctx, s, "count_effective_student_absences", func(ctx context.Context) (careplan.StudentStatusCounts, domain.OperationStats, error) {
		return s.statusDays.CountEffectiveStudentAbsences(ctx, date)
	})
}

func (s *Service) ListStatusDaySummaries(ctx context.Context, from, to careplan.Date) ([]careplan.StatusDaySummary, error) {
	return requestValue(ctx, s, "list_status_day_summaries", func(ctx context.Context) ([]careplan.StatusDaySummary, domain.OperationStats, error) {
		return s.statusDays.ListStatusDaySummaries(ctx, from, to)
	})
}

func (s *Service) ExistingStudentStatusDayIDs(ctx context.Context, ids []int64) ([]int64, error) {
	ids = uniquePositive(ids)
	return requestValue(ctx, s, "existing_student_status_day_ids", func(ctx context.Context) ([]int64, domain.OperationStats, error) {
		return s.statusDays.ExistingStudentStatusDayIDs(ctx, ids)
	})
}

func (s *Service) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (careplan.CarePlanDeletionCounts, error) {
	return requestValue(ctx, s, "count_care_plan_deletion_records", func(ctx context.Context) (careplan.CarePlanDeletionCounts, domain.OperationStats, error) {
		return s.statusDays.CountCarePlanDeletionRecords(ctx, studentID)
	})
}

func (s *Service) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
	return requestCommandValue(s, ctx, "upsert_student_status_day", func(txCtx context.Context) (careplan.StudentStatusDay, domain.OperationStats, error) {
		return s.statusDays.UpsertStudentStatusDay(txCtx, value)
	})
}

func (s *Service) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, clearedAt time.Time, source string) error {
	return requestCommand(s, ctx, "clear_student_status_days", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.statusDays.ClearStudentStatusDays(txCtx, studentID, status, dates, clearedAt, source)
	})
}

func (s *Service) ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	return requestCommand(s, ctx, "clear_student_status_day_by_id", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.statusDays.ClearStudentStatusDayByID(txCtx, id, clearedAt, source)
	})
}

func (s *Service) ArchiveStudentStatusFlags(ctx context.Context, value careplan.StatusFlagArchive) (int64, error) {
	return requestCommandValue(s, ctx, "archive_student_status_flags", func(txCtx context.Context) (int64, domain.OperationStats, error) {
		return s.statusDays.ArchiveStudentStatusFlags(txCtx, value)
	})
}

// DeleteExpiredStudentStatusDays deletes the days dated before the first
// Berlin calendar day a window of retentionDays keeps.
func (s *Service) DeleteExpiredStudentStatusDays(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", careplan.ErrStatusDayRetentionWindow, retentionDays)
	}
	cutoff := careplan.Date(calendar.TodayDate().AddDays(-retentionDays))
	return requestCommandValue(s, ctx, "delete_expired_student_status_days", func(txCtx context.Context) (int, domain.OperationStats, error) {
		deleted, stats, err := s.statusDays.DeleteStudentStatusDaysBefore(txCtx, cutoff)
		return int(deleted), stats, err
	})
}
