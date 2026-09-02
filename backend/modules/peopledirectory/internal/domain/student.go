package domain

import (
	"errors"
	"time"
)

var ErrStudentNotFound = errors.New("student not found")

// StudentStatusAlumnus is the lifecycle status of a graduated child. Rows
// keep it instead of being deleted, so every roster read excludes it.
const StudentStatusAlumnus = "alumnus"

// Student is the directory row behind users.students that other owners are
// allowed to see: identity, class, group, lifecycle, the live absence flags
// and the photo path. Dates are calendar days in YYYY-MM-DD, empty when
// unset.
type Student struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TenantID      int64
	PersonID      int64
	SchoolClass   string
	GroupID       *int64
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
	Sick          *bool
	SickSince     *time.Time
	Excused       *bool
	ExcusedSince  *time.Time
	PhotoPath     *string
}

func (s Student) IsAlumnus() bool { return s.Status == StudentStatusAlumnus }
