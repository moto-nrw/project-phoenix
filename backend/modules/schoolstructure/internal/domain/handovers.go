package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The handovers the substitution module reads and writes. Workforce owns
// education.group_substitution; this is School Structure's view of its rows,
// which the composition root binds over the owner's capability. The teacher
// assignments are School Membership's contract values (#3556).

// The target types of education.group_substitution.
const (
	GroupSubstitutionTypeGroupHandover = "group_handover"
	GroupSubstitutionTypeLegacy        = "legacy_personnel_substitution"
)

// GroupSubstitution is a temporary substitution of a staff member for
// another in a group.
type GroupSubstitution struct {
	ID                int64         `json:"id"`
	TenantID          int64         `json:"tenant_id"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	TargetType        string        `json:"-"`
	GroupID           int64         `json:"group_id"`
	RegularStaffID    *int64        `json:"regular_staff_id,omitempty"`
	SubstituteStaffID int64         `json:"substitute_staff_id"`
	StartDate         calendar.Date `json:"start_date"`
	EndDate           calendar.Date `json:"end_date"`
	Reason            string        `json:"reason,omitempty"`

	// Relations the reads with relations attach.
	Group           *Group             `json:"group,omitempty"`
	RegularStaff    *SubstitutionStaff `json:"regular_staff,omitempty"`
	SubstituteStaff *SubstitutionStaff `json:"substitute_staff,omitempty"`
}

// SubstitutionStaff is a staff member a substitution names, as School
// Membership resolves it. Person stays nil until the People Directory
// resolved the name.
type SubstitutionStaff struct {
	ID       int64               `json:"id"`
	PersonID int64               `json:"person_id"`
	Person   *SubstitutionPerson `json:"person,omitempty"`
}

// SubstitutionPerson is the name of a substitution's staff member.
type SubstitutionPerson struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// FullName returns "Vorname Nachname".
func (p *SubstitutionPerson) FullName() string {
	return p.FirstName + " " + p.LastName
}
