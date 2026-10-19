package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
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

func (e *ExceptionQueryService) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (careplan.StudentStatusDay, error) {
	return requestValue(ctx, e, "find_student_status_day", func(ctx context.Context) (careplan.StudentStatusDay, domain.OperationStats, error) {
		value, found, stats, err := e.statusQueries.FindStudentStatusDay(ctx, id, activeOnly)
		if err == nil && !found {
			err = careplan.ErrStudentStatusDayNotFound
		}
		return value, stats, err
	})
}

func (e *ExceptionQueryService) ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
	return requestValue(ctx, e, "list_student_status_days", func(ctx context.Context) ([]careplan.StudentStatusDay, domain.OperationStats, error) {
		return e.statusQueries.ListStudentStatusDays(ctx, filter)
	})
}

func (e Engine) CountStudentStatusDays(ctx context.Context, options *careplan.StudentScheduleQueryOptions) (int, error) {
	return requestValue(ctx, e, "count_student_status_days", func(ctx context.Context) (int, domain.OperationStats, error) {
		return e.statusDays.CountStudentStatusDays(ctx, options)
	})
}

func (e Engine) CountEffectiveStudentAbsences(ctx context.Context, date careplan.Date) (careplan.StudentStatusCounts, error) {
	return requestValue(ctx, e, "count_effective_student_absences", func(ctx context.Context) (careplan.StudentStatusCounts, domain.OperationStats, error) {
		return e.statusDays.CountEffectiveStudentAbsences(ctx, date)
	})
}

func (e Engine) ListStatusDaySummaries(ctx context.Context, from, to careplan.Date) ([]careplan.StatusDaySummary, error) {
	return requestValue(ctx, e, "list_status_day_summaries", func(ctx context.Context) ([]careplan.StatusDaySummary, domain.OperationStats, error) {
		return e.statusDays.ListStatusDaySummaries(ctx, from, to)
	})
}

func (e Engine) ExistingStudentStatusDayIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return requestValue(ctx, e, "existing_student_status_day_ids", func(ctx context.Context) ([]int64, domain.OperationStats, error) {
		return e.statusDays.ExistingStudentStatusDayIDs(ctx, ids)
	})
}

func (e Engine) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (careplan.CarePlanDeletionCounts, error) {
	return requestValue(ctx, e, "count_care_plan_deletion_records", func(ctx context.Context) (careplan.CarePlanDeletionCounts, domain.OperationStats, error) {
		return e.statusDays.CountCarePlanDeletionRecords(ctx, studentID)
	})
}

func (e Engine) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
	return requestCommandValue(e, ctx, "upsert_student_status_day", func(txCtx context.Context) (careplan.StudentStatusDay, domain.OperationStats, error) {
		return e.statusDays.UpsertStudentStatusDay(txCtx, value)
	})
}

func (e Engine) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, clearedAt time.Time, source string) error {
	return requestCommand(e, ctx, "clear_student_status_days", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.statusDays.ClearStudentStatusDays(txCtx, studentID, status, dates, clearedAt, source)
	})
}

func (e Engine) ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	return requestCommand(e, ctx, "clear_student_status_day_by_id", func(txCtx context.Context) (domain.OperationStats, error) {
		return e.statusDays.ClearStudentStatusDayByID(txCtx, id, clearedAt, source)
	})
}

func (e Engine) ArchiveStudentStatusFlags(ctx context.Context, value careplan.StatusFlagArchive) (int64, error) {
	return requestCommandValue(e, ctx, "archive_student_status_flags", func(txCtx context.Context) (int64, domain.OperationStats, error) {
		return e.statusDays.ArchiveStudentStatusFlags(txCtx, value)
	})
}

func (e Engine) DeleteStudentStatusDaysBefore(ctx context.Context, cutoff careplan.Date) (int, error) {
	return requestCommandValue(e, ctx, "delete_expired_student_status_days", func(txCtx context.Context) (int, domain.OperationStats, error) {
		deleted, stats, err := e.statusDays.DeleteStudentStatusDaysBefore(txCtx, cutoff)
		return int(deleted), stats, err
	})
}
