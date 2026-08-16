package users

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// BirthdayKind separates the two populations a birthday display mixes. They
// are never merged into one list on screen: a child's birthday is celebrated
// with the group, a colleague's is a courtesy note for the team.
type BirthdayKind string

const (
	BirthdayKindStudent BirthdayKind = "student"
	BirthdayKindStaff   BirthdayKind = "staff"
)

// MonthDay is an annually recurring calendar day. A birthday recurs every
// year, so a birthday query matches on month and day and never on the stored
// birth year — modelling it as a timezone.Date would invite exactly that bug.
type MonthDay struct {
	Month time.Month
	Day   int
}

// MonthDayOf reduces a stored birth date to its recurring day.
func MonthDayOf(d timezone.Date) MonthDay {
	return MonthDay{Month: d.Month, Day: d.Day}
}

// BirthdayEntry is one person having a birthday on one of the queried days.
// Birthday carries the full stored date because the age is derived from it for
// children (an OGS wants to know a child turns seven, not that "someone has a
// birthday"); staff-facing consumers drop the year before it reaches a screen.
type BirthdayEntry struct {
	Kind      BirthdayKind  `json:"kind"`
	ID        int64         `json:"id"`
	FirstName string        `json:"first_name"`
	LastName  string        `json:"last_name"`
	Birthday  timezone.Date `json:"birthday"`
	// GroupID is the child's education group. Nil for staff and for children
	// without a group. No longer drives visibility (#2329) — kept because the
	// repository scan populates it alongside GroupName.
	GroupID     *int64 `json:"-"`
	GroupName   string `json:"group_name,omitempty"`
	SchoolClass string `json:"school_class,omitempty"`
}

// FullName is the display name of the person having the birthday.
func (e BirthdayEntry) FullName() string {
	if e.FirstName == "" {
		return e.LastName
	}
	if e.LastName == "" {
		return e.FirstName
	}
	return e.FirstName + " " + e.LastName
}
