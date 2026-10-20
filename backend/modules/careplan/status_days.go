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
	// UnaccountedIDs are the counted active students in neither bucket: the
	// children the dashboard splits into present, at school and at home.
	UnaccountedIDs []int64
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

// StudentStatusDayRetention deletes a school's status days once their
// retention window has passed (#3958). Sick, excused and class-trip days are
// one data kind: the nightly cleanup deletes every day dated before the
// window.
type StudentStatusDayRetention interface {
	DeleteExpiredStudentStatusDays(ctx context.Context, retentionDays int) (int, error)
}

// ErrStatusDayRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrStatusDayRetentionWindow = errors.New("retention window must be positive")
