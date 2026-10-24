package devicefleet

import (
	"time"
)

// Dashboard is the public info-point aggregate. GDPR contract: it carries
// counts and room/activity metadata ONLY — never student names, student IDs,
// photos, or per-child pickup times. The screen rendering it hangs in a
// publicly visible entrance area.
type Dashboard struct {
	// Status is always "active"; inactive displays never reach aggregation.
	Status             string
	SchoolName         string
	DisplayName        string
	ServerTime         time.Time
	Date               time.Time
	RoomOccupancy      []RoomOccupancy
	RunningActivities  []RunningActivity
	UpcomingActivities []UpcomingActivity
	PickupTimes        []PickupBucket
	StudentsPresent    int
	RoomsOccupied      int
	ActivitiesRunning  int
}

// RoomOccupancy is one room with its live student count.
type RoomOccupancy struct {
	Name         string
	GroupName    *string
	CategoryName *string
	StudentCount int
	Capacity     *int
	IsOccupied   bool
}

// RunningActivity is one currently running activity session.
type RunningActivity struct {
	ID           string
	Name         string
	Category     string
	RoomName     string
	Participants int
	MaxCapacity  *int
}

// UpcomingActivity is one planned activity instance later today.
type UpcomingActivity struct {
	ID        string
	Name      string
	Category  string
	StartTime string
	RoomName  string
}

// PickupBucket aggregates how many present students share a pickup time.
// Counts only — never identities.
type PickupBucket struct {
	Time  string
	Count int
}
