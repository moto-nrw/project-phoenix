package enrollment

import (
	"context"
	"encoding/json"
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
	// GuestGroupID is the group the school picked for the week's holiday
	// guests (#4666), nil before it picked one; Guests counts the guests in
	// it whose access has not ended.
	GuestGroupID *int64 `json:"guest_group_id"`
	Guests       int    `json:"guests"`
}

// HolidayParticipant is one confirmed child of a hosted week with the
// minimum fields: name, date of birth, home school and booked time model. A
// nil HomeSchoolID marks a holiday guest; GuestSchoolName is the school the
// parents named for it, if any. SecondGuardian and Answers are what the
// Träger shares beyond them (#5202): the second guardian with name and
// phone, nil without one, and the answers to the shared questions of the
// form version of the child's registration, empty without such questions.
type HolidayParticipant struct {
	ChildID         int64                `json:"child_id"`
	FirstName       string               `json:"first_name"`
	LastName        string               `json:"last_name"`
	BirthDate       calendar.Date        `json:"birth_date"`
	HomeSchoolID    *int64               `json:"home_school_id"`
	GuestSchoolName string               `json:"guest_school_name"`
	TimeModelID     int64                `json:"time_model_id"`
	SecondGuardian  *HolidayHostGuardian `json:"second_guardian"`
	Answers         []HolidayHostAnswer  `json:"answers"`
}

// HolidayHostGuardian is the second guardian of a registration as the
// hosting school sees it: name and phone, never the e-mail address. Phone
// is empty when the parents gave none.
type HolidayHostGuardian struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// HolidayHostAnswer is the answer to a question the Träger shares with the
// hosting school (#5202). Only a question without health information can be
// shared; the database decides that per form version (ADR 0116 addendum).
// Value is a JSON boolean for a yes/no question, the label of the chosen
// option for a selection, a string for every other question ("YYYY-MM-DD"
// for a date), and null when the parents gave no answer.
type HolidayHostAnswer struct {
	Key   string          `json:"key"`
	Label string          `json:"label"`
	Type  FormFieldType   `json:"type"`
	Value json.RawMessage `json:"value"`
}

// HolidayHostQuestion is a shared question of a participant list: one
// column of the list and its export.
type HolidayHostQuestion struct {
	Key   string        `json:"key"`
	Label string        `json:"label"`
	Type  FormFieldType `json:"type"`
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
// Questions are the shared questions its children answered, in the order
// of the form; empty when the period shares none.
type HolidayParticipantList struct {
	Week         HostedHolidayWeek        `json:"week"`
	TimeModels   []HolidayTimeModelCount  `json:"time_models"`
	HomeSchools  []HolidayHomeSchoolCount `json:"home_schools"`
	Questions    []HolidayHostQuestion    `json:"questions"`
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
