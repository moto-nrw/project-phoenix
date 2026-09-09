// Package enrollment defines the People Directory commands consumed by
// enrollment decisions without exposing the full directory interface.
package enrollment

import (
	"context"
	"errors"
	"time"
)

var ErrStudentNotFound = errors.New("student not found")

type Input struct {
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
	GuardianEmail *string
	GuardianPhone *string
}

type CreatedStudent struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	TenantID      int64
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

type Commands interface {
	ReadEnrollmentStudent(context.Context, int64, string) (Record, error)
	LockEnrollmentClassWrites(context.Context) error
	ApplyEnrollmentProfile(context.Context, int64, ProfilePatch) error
	CreateEnrollmentStudent(context.Context, Input) (CreatedStudent, error)
	RenewEnrollmentStudent(context.Context, int64, Input) error
}

// ProfilePatch distinguishes unchanged fields from an explicit NULL.
type ProfilePatch struct {
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
}
