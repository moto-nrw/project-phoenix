package owners

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// StudentStatusDayRepository serves the retained status-day rows over the Care
// Plan capability. The capability persists broad day statuses (sick / excused /
// class trip) and cascades them into per-slot attendance (#1913). Its
// consumers own the ports they need.
type StudentStatusDayRepository struct{ Capability careplan.Capability }

func statusDayFromPublic(value careplan.StudentStatusDay) *absencerecords.StudentStatusDay {
	row := &absencerecords.StudentStatusDay{StudentID: value.StudentID, Date: absencerecords.Date(value.Date), Status: value.Status, ReportedAt: value.ReportedAt, ClearedAt: value.ClearedAt, Source: value.Source, GuardianAccountID: value.GuardianAccountID, Note: value.Note}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func statusDayToPublic(row *absencerecords.StudentStatusDay) careplan.StudentStatusDay {
	return careplan.StudentStatusDay{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: careplan.Date(row.Date), Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
}

func NewStudentStatusDayRepository(capability careplan.Capability) *StudentStatusDayRepository {
	return &StudentStatusDayRepository{Capability: capability}
}

func (r StudentStatusDayRepository) UpsertReported(ctx context.Context, row *absencerecords.StudentStatusDay) error {
	if row == nil {
		return errors.New("student status day cannot be nil")
	}
	value, err := r.Capability.UpsertStudentStatusDay(ctx, statusDayToPublic(row))
	if err == nil {
		*row = *statusDayFromPublic(value)
	}
	return err
}

func (r StudentStatusDayRepository) ArchiveAndClearStatusFlag(ctx context.Context, flagColumn, sinceColumn, status string, date absencerecords.Date, fallback time.Time, source string) (int64, error) {
	return r.Capability.ArchiveStudentStatusFlags(ctx, careplan.StatusFlagArchive{FlagColumn: flagColumn, SinceColumn: sinceColumn, Status: status, Date: careplan.Date(date), ReportedFallback: fallback, Source: source})
}

func (r StudentStatusDayRepository) CountEffectiveDashboardAbsences(ctx context.Context, date absencerecords.Date) (*absencerecords.StudentStatusCounts, error) {
	value, err := r.Capability.CountEffectiveStudentAbsences(ctx, careplan.Date(date))
	if err != nil {
		return nil, err
	}
	return &absencerecords.StudentStatusCounts{Sick: value.Sick, Excused: value.Excused, Total: value.Total, UnaccountedIDs: value.UnaccountedIDs}, nil
}

func (r StudentStatusDayRepository) MarkCleared(ctx context.Context, studentID int64, status string, date absencerecords.Date, at time.Time, source string) error {
	return r.Capability.ClearStudentStatusDays(ctx, studentID, status, []careplan.Date{careplan.Date(date)}, at, source)
}

func (r StudentStatusDayRepository) MarkClearedByID(ctx context.Context, id int64, at time.Time, source string) error {
	return r.Capability.ClearStudentStatusDayByID(ctx, id, at, source)
}

func (r StudentStatusDayRepository) MarkClearedForDates(ctx context.Context, studentID int64, status string, dates []absencerecords.Date, at time.Time, source string) error {
	publicDates := make([]careplan.Date, len(dates))
	for i := range dates {
		publicDates[i] = careplan.Date(dates[i])
	}
	return r.Capability.ClearStudentStatusDays(ctx, studentID, status, publicDates, at, source)
}

func (r StudentStatusDayRepository) FindActiveByID(ctx context.Context, id int64) (*absencerecords.StudentStatusDay, error) {
	value, err := r.Capability.FindStudentStatusDay(ctx, id, true)
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

// FindStandingByStudentIDsAndDateRange returns the status days of the
// students from from to to that still stand: active, or archived at the end
// of their day (#4793). A withdrawn or cleared day is not among them.
func (r StudentStatusDayRepository) FindStandingByStudentIDsAndDateRange(ctx context.Context, studentIDs []int64, from, to absencerecords.Date) ([]*absencerecords.StudentStatusDay, error) {
	if len(studentIDs) == 0 {
		return []*absencerecords.StudentStatusDay{}, nil
	}
	return r.list(ctx, careplan.StudentStatusDayFilter{StudentIDs: studentIDs, From: careplan.Date(from), To: careplan.Date(to), IncludeEndOfDay: true})
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

func (r StudentStatusDayRepository) ListOverview(ctx context.Context, query studentpresence.StatusDayOverviewQuery, orderedStudentIDs []int64) ([]*absencerecords.StudentStatusDay, error) {
	return r.list(ctx, careplan.StudentStatusDayFilter{Options: statusDayOverviewOptions(query), OrderedStudentIDs: orderedStudentIDs, Overview: true})
}

func (r StudentStatusDayRepository) CountOverview(ctx context.Context, query studentpresence.StatusDayOverviewQuery) (int, error) {
	return r.Capability.CountStudentStatusDays(ctx, statusDayOverviewOptions(query))
}

// statusDayOverviewOptions hands the presence overview query to Care Plan,
// which owns what the overview shows.
func statusDayOverviewOptions(query studentpresence.StatusDayOverviewQuery) *careplan.StudentScheduleQueryOptions {
	children := make([]careplan.StatusDayWindow, len(query.Children))
	for i, child := range query.Children {
		children[i] = careplan.StatusDayWindow{StudentID: child.StudentID, From: child.From, Until: child.Until}
	}
	return careplan.StatusDayOverviewOptions(careplan.StatusDayOverview{
		From: query.From, To: query.To, Status: query.Status, Children: children, Page: query.Page, PageSize: query.PageSize,
	})
}

func (r StudentStatusDayRepository) list(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]*absencerecords.StudentStatusDay, error) {
	values, err := r.Capability.ListStudentStatusDays(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*absencerecords.StudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, statusDayFromPublic(value))
	}
	return result, nil
}
