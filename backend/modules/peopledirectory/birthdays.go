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

// BirthdayOverview is the dashboard payload.
type BirthdayOverview struct {
	Enabled      bool
	IncludeStaff bool
	Today        calendar.Date
	Celebrations []BirthdayCelebration
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
	Overview(ctx context.Context, visibility BirthdayVisibility) (BirthdayOverview, error)
	GetOptOut(ctx context.Context, accountID int64) (bool, error)
	SetOptOut(ctx context.Context, accountID int64, optOut bool) error
	ListStaffBirthdays(ctx context.Context, months map[time.Month]bool) ([]StaffBirthday, error)
}
