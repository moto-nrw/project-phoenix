package timetablehttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// People reads the directory facts the timetable routes render: display
// names of staff, accounts and children, and whether a child still attends.
// The composition root binds it over the People Directory person, staff and
// student reads. Every method answers for the request tenant; display names
// are "Vorname Nachname".
type People interface {
	// PersonNames maps person ids to display names; unknown persons are
	// absent.
	PersonNames(ctx context.Context, personIDs []int64) (map[int64]string, error)
	// AccountPersonID returns the id of the account's person. An account
	// without a person is an error.
	AccountPersonID(ctx context.Context, accountID int64) (int64, error)
	// AccountPersonName returns the display name of the account's person. An
	// account without a person is an error.
	AccountPersonName(ctx context.Context, accountID int64) (string, error)
	// StaffPersonID returns the person id of a staff member, 0 when the staff
	// member is unknown.
	StaffPersonID(ctx context.Context, staffID int64) (int64, error)
	// PersonStaffID returns the staff id of a person, 0 when the person is no
	// staff member.
	PersonStaffID(ctx context.Context, personID int64) (int64, error)
	// StaffNames maps staff ids to their person's display name in one read;
	// staff without a person are absent.
	StaffNames(ctx context.Context, staffIDs []int64) (map[int64]string, error)
	// AttendingStudentPersons maps the ids of the children who exist and
	// still attend on day, neither graduated nor past their end of care, to
	// their person ids.
	AttendingStudentPersons(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]int64, error)
	// StudentAttends reports whether the child exists and still attends on
	// day. A missing child may instead return the directory's not-found
	// error.
	StudentAttends(ctx context.Context, studentID int64, day calendar.Date) (bool, error)
}

// readableStudent is the child the read gate is asked about. The directory
// already confirmed that the child exists and still attends.
type readableStudent struct{}

func (readableStudent) IsAuthorizationStudent() bool { return true }
