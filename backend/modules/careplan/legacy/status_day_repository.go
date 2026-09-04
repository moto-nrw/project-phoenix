package legacy

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type studentStatusDayRepository struct{ capability careplan.Capability }

func statusDayFromPublic(value careplan.StudentStatusDay) *activeModels.StudentStatusDay {
	row := &activeModels.StudentStatusDay{StudentID: value.StudentID, Date: timezone.Date(value.Date), Status: value.Status, ReportedAt: value.ReportedAt, ClearedAt: value.ClearedAt, Source: value.Source, GuardianAccountID: value.GuardianAccountID, Note: value.Note}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func statusDayToPublic(row *activeModels.StudentStatusDay) careplan.StudentStatusDay {
	return careplan.StudentStatusDay{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: careplan.Date(row.Date), Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
}

func NewStudentStatusDayRepository(capability careplan.Capability) activeModels.StudentStatusDayOverviewRepository {
	return studentStatusDayRepository{capability: capability}
}

func (r studentStatusDayRepository) UpsertReported(ctx context.Context, row *activeModels.StudentStatusDay) error {
	if row == nil {
		return errors.New("student status day cannot be nil")
	}
	value, err := r.capability.UpsertStudentStatusDay(ctx, statusDayToPublic(row))
	if err == nil {
		*row = *statusDayFromPublic(value)
	}
	return err
}

func (r studentStatusDayRepository) ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date timezone.Date, fallback time.Time, source string) (int64, error) {
	return r.capability.ArchiveStudentStatusFlags(ctx, careplan.StatusFlagArchive{FlagColumn: flagColumn, SinceColumn: sinceColumn, Status: status, Date: careplan.Date(date), ReportedFallback: fallback, Source: source})
}

func (r studentStatusDayRepository) CountEffectiveDashboardAbsences(ctx context.Context, date timezone.Date) (*activeModels.StudentStatusCounts, error) {
	value, err := r.capability.CountEffectiveStudentAbsences(ctx, careplan.Date(date))
	if err != nil {
		return nil, err
	}
	return &activeModels.StudentStatusCounts{Sick: value.Sick, Excused: value.Excused, Total: value.Total}, nil
}

func (r studentStatusDayRepository) MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, at time.Time, source string) error {
	return r.capability.ClearStudentStatusDays(ctx, studentID, status, []careplan.Date{careplan.Date(date)}, at, source)
}

func (r studentStatusDayRepository) MarkClearedByID(ctx context.Context, id int64, at time.Time, source string) error {
	return r.capability.ClearStudentStatusDayByID(ctx, id, at, source)
}

func (r studentStatusDayRepository) MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []timezone.Date, at time.Time, source string) error {
	publicDates := make([]careplan.Date, len(dates))
	for i := range dates {
		publicDates[i] = careplan.Date(dates[i])
	}
	return r.capability.ClearStudentStatusDays(ctx, studentID, status, publicDates, at, source)
}

func (r studentStatusDayRepository) FindActiveByID(ctx context.Context, id int64) (*activeModels.StudentStatusDay, error) {
	value, err := r.capability.FindStudentStatusDay(ctx, id, true)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return statusDayFromPublic(value), nil
}

func (r studentStatusDayRepository) FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: []int64{studentID}, From: careplan.Date(from), To: careplan.Date(to), ActiveOnly: true})
}

func (r studentStatusDayRepository) FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*activeModels.StudentStatusDay{}, nil
	}
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: studentIDs, Date: careplan.Date(date), ActiveOnly: true})
}

func (r studentStatusDayRepository) FindSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*activeModels.StudentStatusDay{}, nil
	}
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: studentIDs, Date: careplan.Date(date), IncludeEndOfDay: true})
}

func (r studentStatusDayRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: []int64{studentID}, From: careplan.Date(from), To: careplan.Date(to)})
}

func (r studentStatusDayRepository) ListOverviewWithOptions(ctx context.Context, options *modelBase.QueryOptions, orderedStudentIDs []int64) ([]*activeModels.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{Options: CarePlanScheduleQueryOptions(options), OrderedStudentIDs: orderedStudentIDs, Overview: true})
}

func (r studentStatusDayRepository) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	return r.capability.CountStudentStatusDays(ctx, CarePlanScheduleQueryOptions(options))
}

func (r studentStatusDayRepository) list(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]*activeModels.StudentStatusDay, error) {
	values, err := r.capability.ListStudentStatusDays(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*activeModels.StudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, statusDayFromPublic(value))
	}
	return result, nil
}
