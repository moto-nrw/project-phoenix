package careplan

import (
	"context"
	"time"
)

// StudentCareProfile holds only Care Plan's fields. MembershipID is obtained
// from School Membership; it is not the public student/profile ID.
type StudentCareProfile struct {
	MembershipID    int64
	SupervisorNotes *string
	HealthInfo      *string
	PickupStatus    *string
	Sick            bool
	SickSince       *time.Time
	Excused         bool
	ExcusedSince    *time.Time
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
