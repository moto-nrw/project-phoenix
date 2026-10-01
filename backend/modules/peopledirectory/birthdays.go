package peopledirectory

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrStaffNotFound reports that the account behind a birthday opt-out has no
// staff record in this tenant: it has nothing to opt out of.
var ErrStaffNotFound = errors.New("staff not found")

// ErrBirthdayWeekOutOfRange reports a requested week further from the current
// one than the display may step (#3777).
var ErrBirthdayWeekOutOfRange = errors.New("birthday week out of range")

// BirthdayKind separates the two populations a birthday display mixes.
type BirthdayKind string

const (
	BirthdayKindStudent BirthdayKind = "student"
	BirthdayKindStaff   BirthdayKind = "staff"
)

// BirthdayCelebration is one person celebrating on one concrete calendar day.
type BirthdayCelebration struct {
	Kind        BirthdayKind
	ID          int64
	Name        string
	GroupName   string
	SchoolClass string
	Date        calendar.Date
	// Age is the age reached; only children carry it, zero means "not disclosed".
	Age     int
	IsToday bool
}

// BirthdayOverview is the dashboard payload: the birthdays of one
// Monday-to-Sunday week and how far the view may step from it.
type BirthdayOverview struct {
	Enabled           bool
	IncludeStaff      bool
	Today             calendar.Date
	WeekStart         calendar.Date
	WeekEnd           calendar.Date
	EarliestWeekStart calendar.Date
	LatestWeekStart   calendar.Date
	Celebrations      []BirthdayCelebration
}

// StaffBirthday is one row of the administrative staff Geburtstagsliste.
type StaffBirthday struct {
	Name     string
	Birthday calendar.Date
}

// BirthdayVisibility carries the caller's student-data decision. A nil value
// means no child is visible.
type BirthdayVisibility interface {
	HasFullAccess() bool
}

// Birthdays is the birthday display capability (#1542): who celebrates and who
// may see it.
type Birthdays interface {
	// Overview returns the week containing weekOf; nil means the current week.
	Overview(ctx context.Context, visibility BirthdayVisibility, weekOf *calendar.Date) (BirthdayOverview, error)
	GetOptOut(ctx context.Context, accountID int64) (bool, error)
	SetOptOut(ctx context.Context, accountID int64, optOut bool) error
	ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]StaffBirthday, error)
}
