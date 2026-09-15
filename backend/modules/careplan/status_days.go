package careplan

import (
	"context"
	"errors"
	"time"
)

var ErrStudentStatusDayNotFound = errors.New("student status day not found")

type StudentStatusDay struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StudentID         int64
	Date              Date
	Status            string
	ReportedAt        time.Time
	ClearedAt         *time.Time
	Source            string
	GuardianAccountID *int64
	Note              *string
}

type StudentStatusDayFilter struct {
	IDs               []int64
	StudentIDs        []int64
	Date              Date
	From              Date
	To                Date
	ActiveOnly        bool
	IncludeEndOfDay   bool
	LatestOnly        bool
	Overview          bool
	Options           *StudentScheduleQueryOptions
	OrderedStudentIDs []int64
}

type StudentStatusCounts struct {
	Sick    int
	Excused int
	Total   int
}

type StatusDaySummary struct {
	StudentID int64
	Date      Date
	Status    string
}

type StatusFlagArchive struct {
	FlagColumn       string
	SinceColumn      string
	Status           string
	Date             Date
	ReportedFallback time.Time
	Source           string
}

type CarePlanDeletionCounts struct {
	StatusDays      int
	ExcusedRequests int
	CareRequests    int
	DataRequests    int
}

type StudentStatusDaysQuery interface {
	FindStudentStatusDay(context.Context, int64, bool) (StudentStatusDay, error)
	ListStudentStatusDays(context.Context, StudentStatusDayFilter) ([]StudentStatusDay, error)
	CountStudentStatusDays(context.Context, *StudentScheduleQueryOptions) (int, error)
	CountEffectiveStudentAbsences(context.Context, Date) (StudentStatusCounts, error)
	ListStatusDaySummaries(context.Context, Date, Date) ([]StatusDaySummary, error)
	ExistingStudentStatusDayIDs(context.Context, []int64) ([]int64, error)
	CountCarePlanDeletionRecords(context.Context, int64) (CarePlanDeletionCounts, error)
}

type StudentStatusDaysCommand interface {
	UpsertStudentStatusDay(context.Context, StudentStatusDay) (StudentStatusDay, error)
	ClearStudentStatusDays(context.Context, int64, string, []Date, time.Time, string) error
	ClearStudentStatusDayByID(context.Context, int64, time.Time, string) error
	ArchiveStudentStatusFlags(context.Context, StatusFlagArchive) (int64, error)
}

func (m *Module) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (StudentStatusDay, error) {
	return m.engine.FindStudentStatusDay(ctx, id, activeOnly)
}

func (m *Module) ListStudentStatusDays(ctx context.Context, filter StudentStatusDayFilter) ([]StudentStatusDay, error) {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StudentIDs = uniquePositive(filter.StudentIDs)
	return m.engine.ListStudentStatusDays(ctx, filter)
}

func (m *Module) CountStudentStatusDays(ctx context.Context, options *StudentScheduleQueryOptions) (int, error) {
	return m.engine.CountStudentStatusDays(ctx, options)
}

func (m *Module) CountEffectiveStudentAbsences(ctx context.Context, date Date) (StudentStatusCounts, error) {
	return m.engine.CountEffectiveStudentAbsences(ctx, date)
}

func (m *Module) ListStatusDaySummaries(ctx context.Context, from, to Date) ([]StatusDaySummary, error) {
	return m.engine.ListStatusDaySummaries(ctx, from, to)
}

func (m *Module) ExistingStudentStatusDayIDs(ctx context.Context, ids []int64) ([]int64, error) {
	return m.engine.ExistingStudentStatusDayIDs(ctx, uniquePositive(ids))
}

func (m *Module) CountCarePlanDeletionRecords(ctx context.Context, studentID int64) (CarePlanDeletionCounts, error) {
	return m.engine.CountCarePlanDeletionRecords(ctx, studentID)
}

func (m *Module) UpsertStudentStatusDay(ctx context.Context, value StudentStatusDay) (StudentStatusDay, error) {
	return m.engine.UpsertStudentStatusDay(ctx, value)
}

func (m *Module) ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []Date, clearedAt time.Time, source string) error {
	return m.engine.ClearStudentStatusDays(ctx, studentID, status, dates, clearedAt, source)
}

func (m *Module) ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string) error {
	return m.engine.ClearStudentStatusDayByID(ctx, id, clearedAt, source)
}

func (m *Module) ArchiveStudentStatusFlags(ctx context.Context, value StatusFlagArchive) (int64, error) {
	return m.engine.ArchiveStudentStatusFlags(ctx, value)
}
