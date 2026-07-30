package active

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// GroupSupervisor represents a staff member assigned to supervise an active group
type GroupSupervisor struct {
	base.Model `bun:"schema:active,table:group_supervisors"`
	base.TenantModel
	StaffID   int64          `bun:"staff_id,notnull" json:"staff_id"`
	GroupID   int64          `bun:"group_id,notnull" json:"group_id"`
	Role      string         `bun:"role,notnull" json:"role"`
	StartDate timezone.Date  `bun:"start_date,notnull,type:date" json:"start_date"`
	EndDate   *timezone.Date `bun:"end_date,type:date" json:"end_date,omitempty"`

	// Relations - these would be populated when using the ORM's relations
	Staff       *users.Staff `bun:"rel:belongs-to,join:staff_id=id" json:"staff,omitempty"`
	ActiveGroup *Group       `bun:"rel:belongs-to,join:group_id=id" json:"active_group,omitempty"`
}

// StaffRoomSupervision is the (staff, room) projection of a currently active
// supervision. It exists so a caller that needs "who supervises which room
// right now" for every staff member of a tenant can ask once, instead of
// pairing FindActiveByStaffID with GetActiveGroupsByIDs per staff member.
type StaffRoomSupervision struct {
	StaffID int64 `bun:"staff_id"`
	RoomID  int64 `bun:"room_id"`
}

// Table name constants for BUN ORM schema qualification
const ()

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
func (gs *GroupSupervisor) SetEndDate(endDate timezone.Date) error {
	if gs.StartDate.After(endDate) {
		return errors.New("end date cannot be before start date")
	}
	gs.EndDate = &endDate
	return nil
}
