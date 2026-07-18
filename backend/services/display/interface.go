// Package display implements the info-point display domain: admin-managed
// screens that render a read-only, token-authenticated dashboard (issue #1325).
package display

import (
	"context"

	displayModels "github.com/moto-nrw/project-phoenix/models/display"
)

// Service defines operations for managing info-point displays and serving
// their public dashboard aggregate.
type Service interface {
	// List returns all displays of the current tenant.
	List(ctx context.Context) ([]*displayModels.Display, error)
	// Create registers a new display and returns it together with the raw
	// access token. The raw token is shown exactly once and never stored.
	Create(ctx context.Context, name string) (*displayModels.Display, string, error)
	// Update changes name and/or active state of a display.
	Update(ctx context.Context, id int64, name *string, isActive *bool) (*displayModels.Display, error)
	// Regenerate mints a new access token for a display, invalidating the
	// old one. Returns the new raw token (shown exactly once).
	Regenerate(ctx context.Context, id int64) (string, error)
	// Delete removes a display permanently.
	Delete(ctx context.Context, id int64) error

	// Dashboard resolves the raw token to a display (admin tx — the token is
	// the only auth signal) and aggregates the dashboard data inside a
	// tenant-scoped transaction. Returns displayModels.ErrNotFound for
	// unknown tokens and displayModels.ErrInactive for deactivated displays.
	Dashboard(ctx context.Context, rawToken string) (*DashboardPayload, error)
}

// DashboardPayload is the public dashboard aggregate. GDPR contract: it
// carries counts and room/activity metadata ONLY — never student names,
// student IDs, photos, or per-child pickup times. The screen rendering this
// hangs in a publicly visible entrance area.
type DashboardPayload struct {
	Status             string             `json:"status"` // always "active" (inactive displays never reach aggregation)
	SchoolName         string             `json:"school_name"`
	DisplayName        string             `json:"display_name"`
	ServerTime         string             `json:"server_time"` // RFC3339, Europe/Berlin
	Date               string             `json:"date"`        // YYYY-MM-DD
	RoomOccupancy      []RoomOccupancy    `json:"room_occupancy"`
	RunningActivities  []RunningActivity  `json:"running_activities"`
	UpcomingActivities []UpcomingActivity `json:"upcoming_activities"`
	PickupTimes        []PickupBucket     `json:"pickup_times"`
	Totals             DashboardTotals    `json:"totals"`
}

// RoomOccupancy is one room with its live student count.
type RoomOccupancy struct {
	Name         string  `json:"name"`
	GroupName    *string `json:"group_name,omitempty"`
	CategoryName *string `json:"category_name,omitempty"`
	StudentCount int     `json:"student_count"`
	Capacity     *int    `json:"capacity,omitempty"`
	IsOccupied   bool    `json:"is_occupied"`
}

// RunningActivity is one currently running activity session.
type RunningActivity struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	RoomName     string `json:"room_name"`
	Participants int    `json:"participants"`
	MaxCapacity  *int   `json:"max_capacity,omitempty"`
}

// UpcomingActivity is one planned activity instance later today.
type UpcomingActivity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	StartTime string `json:"start_time"` // HH:MM wall clock
	RoomName  string `json:"room_name"`
}

// PickupBucket aggregates how many currently present students share the
// same scheduled pickup time today. Counts only — no identities.
type PickupBucket struct {
	Time  string `json:"time"` // HH:MM wall clock
	Count int    `json:"count"`
}

// DashboardTotals carries headline numbers for the screen.
type DashboardTotals struct {
	StudentsPresent   int `json:"students_present"`
	RoomsOccupied     int `json:"rooms_occupied"`
	ActivitiesRunning int `json:"activities_running"`
}
