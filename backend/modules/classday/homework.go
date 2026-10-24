package classday

import (
	"context"
	"errors"
)

// Homework through "moto schule" (#4796): the Lehrkraft enters what an
// assigned class has to do on a day and sees the OGS feedback per child.
// Timetable stores both; the projection maps them onto the day report.

var (
	// ErrHomeworkNotConfigured is returned when the projection was composed
	// without the Timetable homework seam.
	ErrHomeworkNotConfigured = errors.New("homework is not configured")
	// ErrHomeworkInvalid refuses an entry without text, with a text or
	// subject beyond its limit, or without a valid date.
	ErrHomeworkInvalid = errors.New("homework entry is invalid")
	// ErrHomeworkNotFound means the entry does not exist or belongs to
	// another class.
	ErrHomeworkNotFound = errors.New("homework entry not found")
)

// DayHomework is one homework entry of the report's class and date.
type DayHomework struct {
	// ID is serialized as a JSON string: JavaScript clients round numbers
	// beyond 2^53.
	ID          int64  `json:"id,string"`
	Subject     string `json:"subject,omitempty"`
	Description string `json:"description"`
}

// DayHomeworkFeedback is the OGS feedback of one child on the report's date:
// "done", "not_done" or "difficult", with an optional short note.
type DayHomeworkFeedback struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

// HomeworkWrite is a new entry for one class and date. StaffID is the
// author's users.staff_school_memberships row.
type HomeworkWrite struct {
	SchoolClass string
	Date        Date
	Subject     *string
	Description string
	StaffID     int64
}

// HomeworkChange replaces subject and text of one entry of the class,
// whoever entered it.
type HomeworkChange struct {
	ID          int64
	SchoolClass string
	Subject     *string
	Description string
}

// ClassHomework is the homework write seam of moto schule. The caller's
// class assignment is checked before; every entry of the class changes,
// whoever entered it.
type ClassHomework interface {
	AddHomework(ctx context.Context, in HomeworkWrite) (*DayHomework, error)
	ChangeHomework(ctx context.Context, in HomeworkChange) (*DayHomework, error)
	DeleteHomework(ctx context.Context, id int64, schoolClass string) error
}
