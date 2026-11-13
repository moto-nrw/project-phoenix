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
	// ReportedByAccountID is the account that reported the day; nil for a
	// device, a school portal without an account or an automatic run.
	ReportedByAccountID *int64
	// ClearedSource and ClearedByAccountID say who cleared the day. Clearing
	// leaves Source to the report (#6842).
	ClearedSource      *string
	ClearedByAccountID *int64
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
	// SickIDs and ExcusedIDs are the active students in the sick and the
	// excused bucket (a class trip counts as excused), in roster order. The
	// buckets read the status only; a consumer that knows who is checked in
	// drops those children (place before status, #6371).
	SickIDs    []int64
	ExcusedIDs []int64
	Total      int
	// UnaccountedIDs are the counted active students in neither bucket: the
	// children the dashboard splits into present, at school and at home.
	UnaccountedIDs []int64
}

type StatusDaySummary struct {
	StudentID int64
	Date      Date
	Status    string
}

// StatusFlagArchive closes the status days a live sick or excused flag
// stands for. A flag belongs to the status day of its since timestamp
// (Berlin calendar day).
//
// Without Carry the archive covers the flags reported before Before: each
// flag's status day keeps a standing row (an existing row keeps its status
// and source; a day without a row gets an end-of-day row), and the flag is
// cleared. Date is the status day of a flag without a since timestamp.
//
// With Carry the flags reported before Before stay set and stand for Date as
// well: Date gets an active row with the source of the flag's latest earlier
// day, or Source when there is none. A day that already has a row keeps it.
//
// A zero Before takes every flag.
type StatusFlagArchive struct {
	FlagColumn       string
	SinceColumn      string
	Status           string
	Date             Date
	ReportedFallback time.Time
	Source           string
	Before           time.Time
	Carry            bool
}

// CarePlanDeletionCounts is one owner-local read window of the deletion preview.
// ScheduleRows keeps the seven schedule categories separate from status/request rows.
type CarePlanDeletionCounts struct {
	ScheduleRows    int
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
	// The clearing source and the clearing account (nil when no account
	// cleared it) are kept next to the report's source (#6842).
	ClearStudentStatusDays(ctx context.Context, studentID int64, status string, dates []Date, clearedAt time.Time, source string, accountID *int64) error
	ClearStudentStatusDayByID(ctx context.Context, id int64, clearedAt time.Time, source string, accountID *int64) error
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
