package users

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Employment type constants
const (
	EmploymentTypeFullTime = "full_time"
	EmploymentTypePartTime = "part_time"
	EmploymentTypeMinijob  = "minijob"
)

// PersonnelNumberUniqueConstraintName is the partial unique index enforcing
// one Personalnummer per person within a tenant (migration 1.15.229). The
// service layer maps 23505 on it to a stable conflict error.
const PersonnelNumberUniqueConstraintName = "uq_staff_tenant_personnel_number"

// Staff represents a staff member in the system
type Staff struct {
	base.Model `bun:"schema:users,table:staff"`
	base.TenantModel
	PersonID        int64   `bun:"person_id,notnull" json:"person_id"`
	StaffNotes      string  `bun:"staff_notes" json:"staff_notes,omitempty"`
	EmploymentType  *string `bun:"employment_type" json:"employment_type,omitempty"`
	WorkTimeModelID *int64  `bun:"work_time_model_id" json:"work_time_model_id,omitempty"`
	// PersonnelNumber is the payroll system's Personalnummer (DATEV LODAS /
	// Lohn und Gehalt match employees by it, #1417). Unique per tenant via
	// partial index; NULL means "not assigned yet". School-scoped like the
	// row itself — a person at two schools of one Träger should carry the
	// same value in both rows, which RLS cannot enforce (documented gap).
	PersonnelNumber    *string        `bun:"personnel_number" json:"personnel_number,omitempty"`
	RotationAnchorDate *timezone.Date `bun:"rotation_anchor_date,type:date" json:"rotation_anchor_date,omitempty"`
	DeletedAt          *time.Time     `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
}

// Validate ensures staff data is valid
func (s *Staff) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if s.EmploymentType != nil {
		switch *s.EmploymentType {
		case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeMinijob:
			// valid
		default:
			return errors.New("employment_type must be 'full_time', 'part_time', or 'minijob'")
		}
	}

	return nil
}

// IsMinijob returns true if this staff member is a Minijob employee
func (s *Staff) IsMinijob() bool {
	return s.EmploymentType != nil && *s.EmploymentType == EmploymentTypeMinijob
}

// SetPerson links this staff member to a person
func (s *Staff) SetPerson(person *Person) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}

// GetFullName returns the full name of the staff member from the linked person
func (s *Staff) GetFullName() string {
	if s.Person != nil {
		return s.Person.GetFullName()
	}
	return ""
}

// AddNotes adds notes about this staff member
func (s *Staff) AddNotes(notes string) {
	if s.StaffNotes == "" {
		s.StaffNotes = notes
	} else {
		s.StaffNotes += "\n" + notes
	}
}

// StaffWithRoleInfo contains staff data with person info and account details for role-based queries
type StaffWithRoleInfo struct {
	StaffID   int64     `bun:"staff_id" json:"staff_id"`
	CreatedAt time.Time `bun:"created_at" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at" json:"updated_at"`
	PersonID  int64     `bun:"person_id" json:"person_id"`
	FirstName string    `bun:"first_name" json:"first_name"`
	LastName  string    `bun:"last_name" json:"last_name"`
	AccountID int64     `bun:"account_id" json:"account_id"`
	Email     string    `bun:"email" json:"email"`
}

// PIN-related functionality has been moved to auth.Account model
// This simplifies the architecture by centralizing all authentication data
