package active

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Group represents an active group session in a room
type Group struct {
	base.Model `bun:"schema:active,table:groups"`
	base.TenantModel
	StartTime      time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime        *time.Time `bun:"end_time" json:"end_time,omitempty"`
	LastActivity   time.Time  `bun:"last_activity,notnull" json:"last_activity"`      // Activity tracking for timeout
	TimeoutMinutes int        `bun:"timeout_minutes,nullzero" json:"timeout_minutes"` // Session timeout config (default 30)
	GroupID        *int64     `bun:"group_id" json:"group_id"`
	DeviceID       *int64     `bun:"device_id" json:"device_id,omitempty"` // Optional for RFID system
	RoomID         int64      `bun:"room_id,notnull" json:"room_id"`

	// Relations - these would be populated when using the ORM's relations
	ActualGroup *activities.Group  `bun:"rel:belongs-to,join:group_id=id" json:"actual_group,omitempty"`
	Device      *iot.Device        `bun:"rel:belongs-to,join:device_id=id" json:"device,omitempty"`
	Room        *facilities.Room   `bun:"rel:belongs-to,join:room_id=id" json:"room,omitempty"`
	Visits      []*Visit           `bun:"rel:has-many,join:id=active_group_id" json:"visits,omitempty"`
	Supervisors []*GroupSupervisor `bun:"rel:has-many,join:id=group_id" json:"supervisors,omitempty"`
}

// Validate ensures active group data is valid
func (g *Group) Validate() error {
	if g.StartTime.IsZero() {
		return errors.New("start time is required")
	}

	if g.EndTime != nil && g.StartTime.After(*g.EndTime) {
		return errors.New("start time must be before end time")
	}

	// GroupID is optional: a NULL group_id marks a spontaneous activity
	// instance (WP-B6) that runs without a parent template. Only reject
	// non-nil values that are non-positive — those are clearly bad writes.
	if g.GroupID != nil && *g.GroupID <= 0 {
		return errors.New("group ID must be positive when set")
	}

	// DeviceID is now optional for RFID system
	// No validation needed for DeviceID

	if g.RoomID <= 0 {
		return errors.New("room ID is required")
	}

	return nil
}

// IsSpontaneous reports whether this session runs without a parent template
// (i.e. group_id IS NULL). Spontaneous instances are created ad-hoc by staff
// and do not map back to a row in activities.groups.
func (g *Group) IsSpontaneous() bool {
	return g.GroupID == nil
}

// HasTemplate reports whether this session is backed by a template in
// activities.groups. Inverse of IsSpontaneous.
func (g *Group) HasTemplate() bool {
	return g.GroupID != nil
}

// TemplateID returns the parent template's ID and true when the session is
// template-backed, or (0, false) when it is spontaneous. Callers should treat
// this as the only sanctioned way to dereference GroupID — direct pointer
// access is discouraged.
func (g *Group) TemplateID() (int64, bool) {
	if g.GroupID == nil {
		return 0, false
	}
	return *g.GroupID, true
}

// IsActive returns whether this active group session is currently active
func (g *Group) IsActive() bool {
	return g.EndTime == nil
}

// SetEndTime explicitly sets the end time
func (g *Group) SetEndTime(endTime time.Time) error {
	if g.StartTime.After(endTime) {
		return errors.New("end time cannot be before start time")
	}
	g.EndTime = &endTime
	return nil
}

// GetDuration returns the duration of the active group session
func (g *Group) GetDuration() time.Duration {
	if g.EndTime == nil {
		return time.Since(g.StartTime)
	}
	return g.EndTime.Sub(g.StartTime)
}
