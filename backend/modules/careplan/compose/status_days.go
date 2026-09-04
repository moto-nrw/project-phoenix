package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type StatusDayStore interface {
	FindStudentStatusDay(context.Context, int64, bool) (careplan.StudentStatusDay, bool, RequestStoreStats, error)
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, RequestStoreStats, error)
	CountStudentStatusDays(context.Context, *careplan.StudentScheduleQueryOptions) (int, RequestStoreStats, error)
	CountEffectiveStudentAbsences(context.Context, careplan.Date) (careplan.StudentStatusCounts, RequestStoreStats, error)
	ListStatusDaySummaries(context.Context, careplan.Date, careplan.Date) ([]careplan.StatusDaySummary, RequestStoreStats, error)
	ExistingStudentStatusDayIDs(context.Context, []int64) ([]int64, RequestStoreStats, error)
	CountCarePlanDeletionRecords(context.Context, int64) (careplan.CarePlanDeletionCounts, RequestStoreStats, error)
	UpsertStudentStatusDay(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, RequestStoreStats, error)
	ClearStudentStatusDays(context.Context, int64, string, []careplan.Date, time.Time, string) (RequestStoreStats, error)
	ClearStudentStatusDayByID(context.Context, int64, time.Time, string) (RequestStoreStats, error)
	ArchiveStudentStatusFlags(context.Context, careplan.StatusFlagArchive) (int64, RequestStoreStats, error)
}

func (e engine) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (careplan.StudentStatusDay, error) {
	return requestValue(e, "find_student_status_day", func() (careplan.StudentStatusDay, RequestStoreStats, error) {
		value, found, stats, err := e.statusDays.FindStudentStatusDay(ctx, id, activeOnly)
		if err == nil && !found {
			err = careplan.ErrStudentStatusDayNotFound
		}
		return value, stats, err
	})
}

func (e engine) ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
	return requestValue(e, "list_student_status_days", func() ([]careplan.StudentStatusDay, RequestStoreStats, error) {
		return e.statusDays.ListStudentStatusDays(ctx, filter)
	})
}

func (e engine) CountStudentStatusDays(ctx context.Context, options *careplan.StudentScheduleQueryOptions) (int, error) {
	return requestValue(e, "count_student_status_days", func() (int, RequestStoreStats, error) {
		return e.statusDays.CountStudentStatusDays(ctx, options)
	})
}

func (e engine) CountEffectiveStudentAbsences(ctx context.Context, date careplan.Date) (careplan.StudentStatusCounts, error) {
	return requestValue(e, "count_effective_student_absences", func() (careplan.StudentStatusCounts, RequestStoreStats, error) {
		return e.statusDays.CountEffectiveStudentAbsences(ctx, date)
	})
}

func (e engine) ListStatusDaySummaries(ctx context.Context, from, to careplan.Date) ([]careplan.StatusDaySummary, error) {
	return requestValue(e, "list_status_day_summaries", func() ([]careplan.StatusDaySummary, RequestStoreStats, error) {
		return e.statusDays.ListStatusDaySummaries(ctx, from, to)
	})
}

func (e engine) ExistingStudentStatusDayIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return requestValue(e, "existing_student_status_day_ids", func() ([]int64, RequestStoreStats, error) {
		return e.statusDays.ExistingStudentStatusDayIDs(ctx, ids)
	})
}

func (e engine) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (careplan.CarePlanDeletionCounts, error) {
	return requestValue(e, "count_care_plan_deletion_records", func() (careplan.CarePlanDeletionCounts, RequestStoreStats, error) {
		return e.statusDays.CountCarePlanDeletionRecords(ctx, studentID)
	})
}

func (e engine) UpsertStudentStatusDay(ctx context.Context, value careplan.StudentStatusDay) (careplan.StudentStatusDay, error) {
	return requestCommandValue(e, ctx, "upsert_student_status_day", func(txCtx context.Context) (careplan.StudentStatusDay, RequestStoreStats, error) {
		return e.statusDays.UpsertStudentStatusDay(txCtx, value)
	})
}

func (e engine) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []careplan.Date, clearedAt time.Time, source string) error {
	return requestCommand(e, ctx, "clear_student_status_days", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.statusDays.ClearStudentStatusDays(txCtx, studentID, status, dates, clearedAt, source)
	})
}

func (e engine) ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	return requestCommand(e, ctx, "clear_student_status_day_by_id", func(txCtx context.Context) (RequestStoreStats, error) {
		return e.statusDays.ClearStudentStatusDayByID(txCtx, id, clearedAt, source)
	})
}

func (e engine) ArchiveStudentStatusFlags(ctx context.Context, value careplan.StatusFlagArchive) (int64, error) {
	return requestCommandValue(e, ctx, "archive_student_status_flags", func(txCtx context.Context) (int64, RequestStoreStats, error) {
		return e.statusDays.ArchiveStudentStatusFlags(txCtx, value)
	})
}
