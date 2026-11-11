package careplan

import (
	"context"
	"time"
)

// StudentCareProfile holds only Care Plan's fields. MembershipID is obtained
// from School Membership; it is not the public student/profile ID.
type StudentCareProfile struct {
	MembershipID int64
	HealthInfo   *string
	PickupStatus *string
	Sick         bool
	SickSince    *time.Time
	Excused      bool
	ExcusedSince *time.Time
}

type StudentDeparturePlan struct {
	MembershipID          int64
	PickupStatus          string
	AllowedDepartureModes map[string][]string
	DepartureDays         map[string]string
	BusDays               map[string]bool
	PickupDays            map[string]bool
	CompanionNote         *string
	PlanTouched           bool
}

type StudentProfileCommands interface {
	SetStudentLiveStatus(context.Context, StudentLiveStatus) (int64, error)
	// SetStudentHealthInfo writes health_info (nil clears it) on the child's
	// care row in the current tenant and returns the rows changed; 0 means the
	// child has no care row there.
	SetStudentHealthInfo(ctx context.Context, studentID int64, healthInfo *string) (int64, error)
	ClearStudentStatusFlags(context.Context, []int64, string) (int64, error)
	SaveStudentCareProfile(context.Context, StudentCareProfile, *StudentDeparturePlan) error
}

type StudentLiveStatus struct {
	StudentID    int64
	Sick         bool
	SickSince    *time.Time
	Excused      bool
	ExcusedSince *time.Time
}

// StudentCareProfileSave is one care profile of a new membership a batch
// saves, with its departure plan as SaveStudentCareProfile takes them.
type StudentCareProfileSave struct {
	Profile StudentCareProfile
	Plan    *StudentDeparturePlan
}

// StudentCareProfileBatches saves the care profiles of many new memberships
// at once (#5552, ADR 0067 addendum of 2026-11-08, point 3:
// SaveStudentCareProfiles, called by People Directory's enrollment batch). It
// is a leaf command outside StudentProfileCommands; the root binding asserts
// it.
type StudentCareProfileBatches interface {
	// SaveStudentCareProfiles inserts every care profile with the columns
	// and values SaveStudentCareProfile writes for a membership without one,
	// in the caller's transaction, all or none. A membership that has a
	// care profile already fails the call.
	SaveStudentCareProfiles(context.Context, []StudentCareProfileSave) error
}
