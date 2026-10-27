package enrollment

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Participant list of the school that hosts a holiday week (spec #4611 user
// stories 41 and 42, #4665, ADR 0116): before and during its week the hosting
// OGS sees the confirmed children of the week with the time model they booked
// and their home school, and how many children each time model and each home
// school brings. The access comes only from a confirmed booking, covers only
// the child, the week and the minimum fields, and ends after the last day of
// the week. Every operation runs in the tenant transaction of the hosting
// school, which the caller opens; the database lets that transaction read
// only these rows (ADR 0116).

// ErrHostedHolidayWeekNotFound reports a week the school does not host, or
// one whose last day has passed: the access to its participants has ended.
var ErrHostedHolidayWeekNotFound = errors.New("hosted holiday week not found")

// HolidayHost is the hosting school of the session and the account that
// reads its participant list, with the role the access log names.
type HolidayHost struct {
	SchoolID  int64
	AccountID int64
	Role      string
}

// HostedHolidayWeek is a week the school hosts whose last day has not passed,
// with the number of confirmed children.
type HostedHolidayWeek struct {
	ID           int64         `json:"id"`
	PeriodName   string        `json:"period_name"`
	StartsOn     calendar.Date `json:"starts_on"`
	EndsOn       calendar.Date `json:"ends_on"`
	Participants int           `json:"participants"`
}

// HolidayParticipant is one confirmed child of a hosted week with the
// minimum fields: name, date of birth, home school and booked time model. A
// nil HomeSchoolID marks a holiday guest; GuestSchoolName is the school the
// parents named for it, if any.
type HolidayParticipant struct {
	ChildID         int64         `json:"child_id"`
	FirstName       string        `json:"first_name"`
	LastName        string        `json:"last_name"`
	BirthDate       calendar.Date `json:"birth_date"`
	HomeSchoolID    *int64        `json:"home_school_id"`
	GuestSchoolName string        `json:"guest_school_name"`
	TimeModelID     int64         `json:"time_model_id"`
}

// HolidayTimeModelCount is a time model of the week with the number of
// confirmed children who booked it.
type HolidayTimeModelCount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Count    int    `json:"count"`
}

// HolidayHomeSchoolCount is the number of confirmed children of one home
// school; a nil SchoolID counts the holiday guests.
type HolidayHomeSchoolCount struct {
	SchoolID *int64 `json:"school_id"`
	Count    int    `json:"count"`
}

// HolidayParticipantList is the participant list of one hosted week.
type HolidayParticipantList struct {
	Week         HostedHolidayWeek        `json:"week"`
	TimeModels   []HolidayTimeModelCount  `json:"time_models"`
	HomeSchools  []HolidayHomeSchoolCount `json:"home_schools"`
	Participants []HolidayParticipant     `json:"participants"`
}

// HolidayParticipantAccess is one read of a participant list for the access
// log of the hosting school. Format is empty for the list and names the
// file format of an export.
type HolidayParticipantAccess struct {
	ActorAccountID int64
	ActorRole      string
	WeekID         int64
	StartsOn       calendar.Date
	EndsOn         calendar.Date
	Participants   int
	Format         string
}

// HolidayParticipantAccessLog writes a read of a participant list into the
// access log of the hosting school, in the caller's tenant transaction. The
// root binds it to the Audit Platform.
type HolidayParticipantAccessLog interface {
	RecordHolidayParticipantAccess(ctx context.Context, access HolidayParticipantAccess) error
}

// HolidayHostParticipants serves the participant lists to the hosting
// school. Every operation runs in the tenant transaction of host.SchoolID.
type HolidayHostParticipants interface {
	// HostedHolidayWeeks lists the weeks of published periods the school
	// hosts whose last day has not passed, oldest first. It reads no
	// personal data.
	HostedHolidayWeeks(ctx context.Context, host HolidayHost) ([]HostedHolidayWeek, error)
	// HolidayParticipants returns the participant list of one hosted week
	// and records the read in the school's access log; a failed entry fails
	// the read.
	HolidayParticipants(ctx context.Context, host HolidayHost, weekID int64) (HolidayParticipantList, error)
	// ExportHolidayParticipants returns the same list for an export file
	// and records the export with its format.
	ExportHolidayParticipants(ctx context.Context, host HolidayHost, weekID int64, format string) (HolidayParticipantList, error)
}

// HolidayHostSchoolNames names the schools of the hosting school's Träger,
// deleted schools included, so the participant list can name a home school.
// The root binds it to Organisation & Tenancy; the route asks before it
// opens the tenant transaction (ADR 0116).
type HolidayHostSchoolNames interface {
	HolidayHostSchoolNames(ctx context.Context, schoolID int64) (map[int64]string, error)
}
