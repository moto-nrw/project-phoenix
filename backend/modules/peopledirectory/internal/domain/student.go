package domain

import (
	"errors"
	"time"
)

var ErrStudentNotFound = errors.New("student not found")

// EnrollmentProfilePatch distinguishes unchanged fields from an explicit NULL.
type EnrollmentProfilePatch struct {
	// DepartureSet replaces the normalized plan and its legacy mirrors together.
	DepartureSet                bool
	DepartureCompanionDays      map[string]bool
	AllowedDepartureModes       map[string][]string
	DepartureDays               map[string]string
	BusDays                     map[string]bool
	PickupDays                  map[string]bool
	PickupStatus                string
	DepartureCompanionNote      *string
	HealthInfoSet               bool
	HealthInfo                  *string
	ExtraInfoSet                bool
	ExtraInfo                   *string
	PhotoConsentGivenAtSet      bool
	PhotoConsentGivenAt         *time.Time
	PhotoConsentGivenBySet      bool
	PhotoConsentGivenBy         *int64
	AGBAcceptedAtSet            bool
	AGBAcceptedAt               *time.Time
	DataProcessingAcceptedAtSet bool
	DataProcessingAcceptedAt    *time.Time
	EmailContactAcceptedAtSet   bool
	EmailContactAcceptedAt      *time.Time
	// The data import patches the directory columns below; enrollment
	// decisions leave them unset.
	GroupIDSet         bool
	GroupID            *int64
	AddressSet         bool
	AddressStreet      *string
	AddressCity        *string
	AddressPostalCode  *string
	SupervisorNotesSet bool
	SupervisorNotes    *string
}

type EnrollmentStudent struct {
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
	GuardianEmail *string
	GuardianPhone *string
}

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

type StudentName struct {
	StudentID int64
	FirstName string
	LastName  string
}

func (s Student) IsAlumnus() bool { return s.Status == StudentStatusAlumnus }
