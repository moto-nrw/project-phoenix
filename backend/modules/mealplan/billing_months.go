package mealplan

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Mensa-Monatsfreigabe (#4686, ADR 0166): at the end of a month the
// OGS-Leitung checks who ate and releases the month. A meal counts on a day
// of the month that is a school day (no weekend, closing day, public holiday
// or school holiday of the school calendar), on which the child is
// registered for lunch after every cancellation of its parents (the meal
// registration of #3873) and on which its attendance is on file. The
// release keeps these counts per child and never changes; only a released
// month goes into the Träger's monthly run.
//
// Invariants:
//   - A month is released once (uq_meal_billing_months_month) and only
//     after its last day.
//   - The release stores what the OGS-Leitung saw: it refuses when the
//     children or meals differ from the counts the request names.
//   - The Träger reads a release only in its own Träger transaction and only
//     the fields a bill needs (schedule.traeger_meal_billing_children).

// BillingMonthStatus is where a month of the Mensa billing stands.
type BillingMonthStatus string

const (
	// BillingMonthRunning: the month has not ended; it cannot be released.
	BillingMonthRunning BillingMonthStatus = "running"
	// BillingMonthOpen: the month has ended and waits for the release.
	BillingMonthOpen BillingMonthStatus = "open"
	// BillingMonthReleased: the OGS-Leitung released the month.
	BillingMonthReleased BillingMonthStatus = "released"
	// BillingMonthExpired: the month ended without a release and its
	// attendance may be deleted under the school's retention; nobody can
	// release or bill it any more.
	BillingMonthExpired BillingMonthStatus = "expired"
)

// BillingMonthChild is one child of a month: the school days it was
// registered for lunch, the meals counted and, before the release, the
// registered days it was not present (Fehlzeiten).
type BillingMonthChild struct {
	StudentID      int64  `json:"student_id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	SchoolClass    string `json:"school_class"`
	RegisteredDays int    `json:"registered_days"`
	MealDays       int    `json:"meal_days"`
	AbsentDates    []Date `json:"absent_dates"`
}

// BillingMonth is a month as the OGS-Leitung checks it: before the release
// computed from the registration and the attendance of today, after it the
// stored counts. Absent dates are only shown before the release.
type BillingMonth struct {
	Month        Date                `json:"month"`
	Status       BillingMonthStatus  `json:"status"`
	SchoolDays   int                 `json:"school_days"`
	ReleasedAt   *time.Time          `json:"released_at"`
	ChildCount   int                 `json:"child_count"`
	MealCount    int                 `json:"meal_count"`
	AbsenceCount int                 `json:"absence_count"`
	Children     []BillingMonthChild `json:"children"`
}

// BillingMonthSummary is a month of the overview without children.
type BillingMonthSummary struct {
	Month      Date               `json:"month"`
	Status     BillingMonthStatus `json:"status"`
	ReleasedAt *time.Time         `json:"released_at"`
	ChildCount *int               `json:"child_count"`
	MealCount  *int               `json:"meal_count"`
}

// BillingMonthRelease names what the OGS-Leitung saw when it released.
type BillingMonthRelease struct {
	Month      Date
	AccountID  int64
	ChildCount int
	MealCount  int
}

// ReleasedMealChild is a child of a release as the Träger reads it: the
// meals counted, no registered day and no absence.
type ReleasedMealChild struct {
	StudentID int64
	FirstName string
	LastName  string
	MealDays  int
}

// ReleasedMealMonth is a released month of a school of the Träger.
type ReleasedMealMonth struct {
	SchoolID   int64
	Month      calendar.Date
	ReleasedAt time.Time
	Children   []ReleasedMealChild
}

var (
	// ErrBillingMonthRunning refuses the release of a month before its end.
	ErrBillingMonthRunning = &Fault{Code: "meal_billing_month_running", Detail: "the month has not ended yet"}
	// ErrBillingMonthReleased refuses a second release of a month.
	ErrBillingMonthReleased = &Fault{Code: "meal_billing_month_released", Detail: "the month is already released"}
	// ErrBillingMonthChanged refuses a release whose counts differ from the
	// month now; the OGS-Leitung checks it again.
	ErrBillingMonthChanged = &Fault{Code: "meal_billing_month_changed", Detail: "the month changed since it was read"}
	// ErrBillingMonthAttendanceExpired refuses counts whose attendance may
	// already have been removed by retention.
	ErrBillingMonthAttendanceExpired = &Fault{Code: "meal_billing_month_attendance_expired", Detail: "the month is outside the attendance retention window"}
	// ErrBillingMonthNoTraeger refuses a release of a school without a Träger:
	// nobody bills its Mensa.
	ErrBillingMonthNoTraeger = &Fault{Code: "meal_billing_month_no_traeger", Detail: "the school belongs to no Träger"}
	// ErrInvalidBillingMonth refuses a month that is not the first day of a
	// month or lies more than a year back.
	ErrInvalidBillingMonth = &Fault{Code: "meal_billing_month_invalid", Detail: "month must be the first day of one of the last twelve months"}
)

// PresenceFinder names per day of [from, to] the children with an
// attendance on file in the caller's school transaction.
type PresenceFinder func(ctx context.Context, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error)

// AttendanceAvailableFrom names the earliest day whose attendance is
// still complete under the school's retention policy.
type AttendanceAvailableFrom func(ctx context.Context) (calendar.Date, error)

// BillingMonths is the Mensa-Monatsfreigabe of a school. Every operation
// runs in the caller's school transaction.
type BillingMonths interface {
	BillingMonthOverview(ctx context.Context) ([]BillingMonthSummary, error)
	BillingMonth(ctx context.Context, month Date) (BillingMonth, error)
	ReleaseBillingMonth(ctx context.Context, release BillingMonthRelease) (BillingMonth, error)
}

// TraegerMealMonths is the Träger's read of the released Mensa months of
// its schools. It runs in the Träger transaction of organizationID and
// answers nothing in any other.
type TraegerMealMonths interface {
	ReleasedMealMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]ReleasedMealMonth, error)
}
