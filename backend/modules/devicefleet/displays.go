package devicefleet

import (
	"context"
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

// ListDisplays returns every info-point screen of the caller's tenant.
func (m *Module) ListDisplays(ctx context.Context) ([]Display, error) {
	return m.engine.ListDisplays(ctx)
}

// CreateDisplay registers a screen and returns it with its raw access token.
// The raw token is returned exactly once and is never stored or logged.
func (m *Module) CreateDisplay(ctx context.Context, name string) (Display, string, error) {
	return m.engine.CreateDisplay(ctx, name)
}

// UpdateDisplay changes the name and/or the active state of one screen.
func (m *Module) UpdateDisplay(ctx context.Context, id int64, name *string, isActive *bool) (Display, error) {
	if id <= 0 {
		return Display{}, m.reject("update_display", ErrInvalidDisplayInput)
	}
	return m.engine.UpdateDisplay(ctx, id, name, isActive)
}

// RegenerateDisplayToken mints a new access token, invalidating the old one.
func (m *Module) RegenerateDisplayToken(ctx context.Context, id int64) (string, error) {
	if id <= 0 {
		return "", m.reject("regenerate_display_token", ErrInvalidDisplayInput)
	}
	return m.engine.RegenerateDisplayToken(ctx, id)
}

// DeleteDisplay removes one screen permanently.
func (m *Module) DeleteDisplay(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_display", ErrInvalidDisplayInput)
	}
	return m.engine.DeleteDisplay(ctx, id)
}

// Dashboard resolves a raw display token to the public aggregate. Unknown,
// revoked, offboarded, and feature-disabled screens all return
// ErrDisplayNotFound; a known but deactivated screen returns
// ErrDisplayInactive.
func (m *Module) Dashboard(ctx context.Context, rawToken string) (Dashboard, error) {
	if rawToken == "" {
		return Dashboard{}, m.reject("display_dashboard", ErrDashboardTokenMissing)
	}
	return m.engine.Dashboard(ctx, rawToken)
}
