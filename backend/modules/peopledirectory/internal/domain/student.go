package domain

import (
	"errors"
	"time"
)

var (
	ErrStudentNotFound = errors.New("student not found")
	// ErrStudentLockBusy reports a NOWAIT row lock that another transaction
	// holds. Callers retry instead of waiting head-on against that writer.
	ErrStudentLockBusy = errors.New("student row is locked by another transaction")
)

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
	GroupIDSet        bool
	GroupID           *int64
	AddressSet        bool
	AddressStreet     *string
	AddressCity       *string
	AddressPostalCode *string
	// PermanentHint is the import's „Betreuernotizen" cell. A non-empty text
	// becomes the child's carried-over permanent hint; an empty one leaves
	// the note card alone.
	PermanentHint *string
}

type EnrollmentStudent struct {
	InitialProfile *EnrollmentProfilePatch
	PersonID       int64
	SchoolClass    string
	Status         string
	EnrolledFrom   string
	EnrolledUntil  string
}

// StudentStatusAlumnus is the lifecycle status of a graduated child. Rows
// keep it instead of being deleted, so every roster read excludes it.
const StudentStatusAlumnus = "alumnus"

// StudentStatusActive is the lifecycle status of a currently enrolled child.
// It is also the column's default, which is why an unset status means this.
const StudentStatusActive = "active"
