package ports

import (
	"errors"
	"time"
)

// ActiveGroup is a room session (one active.groups row) together with the
// display relations the presence services read it with. The relations are
// projections other owners provide; the session carries only their IDs.
// The JSON shape is the wire shape of the caller-context session routes.
type ActiveGroup struct {
	ID             int64      `json:"id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	TenantID       int64      `json:"tenant_id"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	LastActivity   time.Time  `json:"last_activity"`   // Activity tracking for timeout
	TimeoutMinutes int        `json:"timeout_minutes"` // Session timeout config (default 30)
	GroupID        *int64     `json:"group_id"`
	DeviceID       *int64     `json:"device_id,omitempty"` // Optional for RFID system
	RoomID         int64      `json:"room_id"`

	// ActualGroup and Room are projections populated through their owners.
	ActualGroup *SessionActivity `json:"actual_group,omitempty"`
	// Device is resolved through the Device Fleet owner (#2676), never by a
	// join this module owns.
	Device      *SessionDevice     `json:"device,omitempty"`
	Room        *SessionRoom       `json:"room,omitempty"`
	Supervisors []*GroupSupervisor `json:"supervisors,omitempty"`
}

// SessionDevice is the device information projected onto a presence session.
// Device Fleet owns the device record; session reads do not expose credentials
// or mutable device configuration.
type SessionDevice struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenant_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeviceID   string     `json:"device_id"`
	DeviceType string     `json:"device_type"`
	Name       *string    `json:"name,omitempty"`
	Status     string     `json:"status"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
}

// GetID returns the session ID.
func (g *ActiveGroup) GetID() int64 { return g.ID }

// GetTenantID returns the owning tenant.
func (g *ActiveGroup) GetTenantID() int64 { return g.TenantID }

// SetTenantID sets the owning tenant.
func (g *ActiveGroup) SetTenantID(id int64) { g.TenantID = id }

// Validate ensures active group data is valid
func (g *ActiveGroup) Validate() error {
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

	// DeviceID is optional for the RFID system; no validation needed.

	if g.RoomID <= 0 {
		return errors.New("room ID is required")
	}

	return nil
}

// IsSpontaneous reports whether this session runs without a parent template
// (i.e. group_id IS NULL). Spontaneous instances are created ad-hoc by staff
// and do not map back to a row in activities.groups.
func (g *ActiveGroup) IsSpontaneous() bool {
	return g.GroupID == nil
}

// HasTemplate reports whether this session is backed by a template in
// activities.groups. Inverse of IsSpontaneous.
func (g *ActiveGroup) HasTemplate() bool {
	return g.GroupID != nil
}

// IsIndependentRoomSession reports a device-less stay under a system
// activity: the room's own session (#3066), not occupancy of an activity
// running there. activityIsSystem is the linked template's is_system flag.
func (g *ActiveGroup) IsIndependentRoomSession(activityIsSystem bool) bool {
	return g.DeviceID == nil && g.HasTemplate() && activityIsSystem
}

// TemplateID returns the parent template's ID and true when the session is
// template-backed, or (0, false) when it is spontaneous. Callers should treat
// this as the only sanctioned way to dereference GroupID — direct pointer
// access is discouraged.
func (g *ActiveGroup) TemplateID() (int64, bool) {
	if g.GroupID == nil {
		return 0, false
	}
	return *g.GroupID, true
}

// IsActive returns whether this active group session is currently active
func (g *ActiveGroup) IsActive() bool {
	return g.EndTime == nil
}

// SetEndTime explicitly sets the end time
func (g *ActiveGroup) SetEndTime(endTime time.Time) error {
	if g.StartTime.After(endTime) {
		return errors.New("end time cannot be before start time")
	}
	g.EndTime = &endTime
	return nil
}

// GetDuration returns the duration of the active group session
func (g *ActiveGroup) GetDuration() time.Duration {
	if g.EndTime == nil {
		return time.Since(g.StartTime)
	}
	return g.EndTime.Sub(g.StartTime)
}

// GroupSupervisor is a staff member assigned to supervise a room session (one
// active.group_supervisors row).
type GroupSupervisor struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`
	StaffID   int64     `json:"staff_id"`
	GroupID   int64     `json:"group_id"`
	Role      string    `json:"role"`
	StartDate Date      `json:"start_date"`
	EndDate   *Date     `json:"end_date,omitempty"`

	// Owner-provided projections, not ORM relations.
	Staff       *SessionStaff `json:"staff,omitempty"`
	ActiveGroup *ActiveGroup  `json:"active_group,omitempty"`
}

// GetID returns the supervision ID.
func (gs *GroupSupervisor) GetID() int64 { return gs.ID }

// GetTenantID returns the owning tenant.
func (gs *GroupSupervisor) GetTenantID() int64 { return gs.TenantID }

// SetTenantID sets the owning tenant.
func (gs *GroupSupervisor) SetTenantID(id int64) { gs.TenantID = id }

// Validate ensures group supervisor data is valid
func (gs *GroupSupervisor) Validate() error {
	if gs.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if gs.GroupID <= 0 {
		return errors.New("group ID is required")
	}

	if gs.Role == "" {
		return errors.New("role is required")
	}

	if gs.StartDate.IsZero() {
		return errors.New("start date is required")
	}

	if gs.EndDate != nil && gs.StartDate.After(*gs.EndDate) {
		return errors.New("start date must be before end date")
	}

	return nil
}

// SetEndDate explicitly sets the end date
func (gs *GroupSupervisor) SetEndDate(endDate Date) error {
	if gs.StartDate.After(endDate) {
		return errors.New("end date cannot be before start date")
	}
	gs.EndDate = &endDate
	return nil
}

// SupervisionBlocker is an open supervision projected for capability checks.
type SupervisionBlocker struct {
	ID        int64
	GroupID   int64
	GroupName string
	StartDate string
}

// CrossTenantStudent represents the minimal student data exposed
// when a hosting tenant requests information about a visiting student
// from another tenant. Only essential fields are included per GDPR
// data minimization (Datensparsamkeit) requirements.
//
// Used by the Ferienbetreuung (holiday care) feature where students
// from one school temporarily attend another school's program.
type CrossTenantStudent struct {
	StudentID    int64  `json:"student_id"`
	PersonID     int64  `json:"-"`
	HomeTenantID int64  `json:"-"`
	GroupID      *int64 `json:"-"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	GroupName    string `json:"group_name"`
	HomeTenant   string `json:"home_tenant"` // slug of the student's home school
}
