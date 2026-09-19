package domain

import "time"

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

type StudentLiveStatus struct {
	StudentID    int64
	Sick         bool
	SickSince    *time.Time
	Excused      bool
	ExcusedSince *time.Time
}
