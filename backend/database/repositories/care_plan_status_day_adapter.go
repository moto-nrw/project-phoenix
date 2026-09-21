package repositories

import (
	"context"
	"errors"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// StudentStatusDayRepository serves the retained status-day rows over the Care
// Plan capability. The capability persists broad day statuses (sick / excused /
// class trip) and cascades them into per-slot attendance (#1913). Its
// consumers own the ports they need.
type StudentStatusDayRepository struct{ capability careplan.Capability }

func statusDayFromPublic(value careplan.StudentStatusDay) *absencerecords.StudentStatusDay {
	row := &absencerecords.StudentStatusDay{StudentID: value.StudentID, Date: absencerecords.Date(value.Date), Status: value.Status, ReportedAt: value.ReportedAt, ClearedAt: value.ClearedAt, Source: value.Source, GuardianAccountID: value.GuardianAccountID, Note: value.Note}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func statusDayToPublic(row *absencerecords.StudentStatusDay) careplan.StudentStatusDay {
	return careplan.StudentStatusDay{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: careplan.Date(row.Date), Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
}

func NewStudentStatusDayRepository(capability careplan.Capability) *StudentStatusDayRepository {
	return &StudentStatusDayRepository{capability: capability}
}

func (r StudentStatusDayRepository) UpsertReported(ctx context.Context, row *absencerecords.StudentStatusDay) error {
	if row == nil {
		return errors.New("student status day cannot be nil")
	}
	value, err := r.capability.UpsertStudentStatusDay(ctx, statusDayToPublic(row))
	if err == nil {
		*row = *statusDayFromPublic(value)
	}
	return err
}

func (r StudentStatusDayRepository) ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date absencerecords.Date, fallback time.Time, source string) (int64, error) {
	return r.capability.ArchiveStudentStatusFlags(ctx, careplan.StatusFlagArchive{FlagColumn: flagColumn, SinceColumn: sinceColumn, Status: status, Date: careplan.Date(date), ReportedFallback: fallback, Source: source})
}

func (r StudentStatusDayRepository) CountEffectiveDashboardAbsences(ctx context.Context, date absencerecords.Date) (*absencerecords.StudentStatusCounts, error) {
	value, err := r.capability.CountEffectiveStudentAbsences(ctx, careplan.Date(date))
	if err != nil {
		return nil, err
	}
	return &absencerecords.StudentStatusCounts{Sick: value.Sick, Excused: value.Excused, Total: value.Total, UnaccountedIDs: value.UnaccountedIDs}, nil
}

func (r StudentStatusDayRepository) MarkCleared(ctx context.Context, studentID int64, status string, date absencerecords.Date, at time.Time, source string) error {
	return r.capability.ClearStudentStatusDays(ctx, studentID, status, []careplan.Date{careplan.Date(date)}, at, source)
}

func (r StudentStatusDayRepository) MarkClearedByID(ctx context.Context, id int64, at time.Time, source string) error {
	return r.capability.ClearStudentStatusDayByID(ctx, id, at, source)
}

func (r StudentStatusDayRepository) MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []absencerecords.Date, at time.Time, source string) error {
	publicDates := make([]careplan.Date, len(dates))
	for i := range dates {
		publicDates[i] = careplan.Date(dates[i])
	}
	return r.capability.ClearStudentStatusDays(ctx, studentID, status, publicDates, at, source)
}

func (r StudentStatusDayRepository) FindActiveByID(ctx context.Context, id int64) (*absencerecords.StudentStatusDay, error) {
	value, err := r.capability.FindStudentStatusDay(ctx, id, true)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, noRowsError()
	}
	if err != nil {
		return nil, err
	}
	return statusDayFromPublic(value), nil
}

func (r StudentStatusDayRepository) FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, from, to absencerecords.Date) ([]*absencerecords.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: []int64{studentID}, From: careplan.Date(from), To: careplan.Date(to), ActiveOnly: true})
}

func (r StudentStatusDayRepository) FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date absencerecords.Date) ([]*absencerecords.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*absencerecords.StudentStatusDay{}, nil
	}
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: studentIDs, Date: careplan.Date(date), ActiveOnly: true})
}

func (r StudentStatusDayRepository) FindSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date absencerecords.Date) ([]*absencerecords.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*absencerecords.StudentStatusDay{}, nil
	}
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: studentIDs, Date: careplan.Date(date), IncludeEndOfDay: true})
}

func (r StudentStatusDayRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to absencerecords.Date) ([]*absencerecords.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: []int64{studentID}, From: careplan.Date(from), To: careplan.Date(to)})
}

func (r StudentStatusDayRepository) ListOverviewWithOptions(ctx context.Context, options *userModels.QueryOptions, orderedStudentIDs []int64) ([]*absencerecords.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{Options: legacyScheduleQueryOptions(options), OrderedStudentIDs: orderedStudentIDs, Overview: true})
}

func (r StudentStatusDayRepository) CountWithOptions(ctx context.Context, options *userModels.QueryOptions) (int, error) {
	return r.capability.CountStudentStatusDays(ctx, legacyScheduleQueryOptions(options))
}

func (r StudentStatusDayRepository) list(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]*absencerecords.StudentStatusDay, error) {
	values, err := r.capability.ListStudentStatusDays(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*absencerecords.StudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, statusDayFromPublic(value))
	}
	return result, nil
}
