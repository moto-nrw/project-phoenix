package users

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// ErrPersonnelNumberConflict is the repository-level classification of that
// duplicate. The staff repository is a composition over the School Membership
// owner, which classifies the unique violation itself, so the service layer
// matches on this sentinel instead of the raw 23505 it can no longer see.
var ErrPersonnelNumberConflict = contact.ErrPersonnelNumberConflict

// Staff represents a staff member in the system. It is a composed DTO, not a
// table: School Membership (users.staff_school_memberships) and Workforce
// (users.staff_employment_profiles) own its columns since #2753, and the old
// users.staff table is gone since #2754.
type Staff struct {
	Model
	TenantModel
	PersonID        int64   `bun:"person_id,notnull" json:"person_id"`
	StaffNotes      string  `bun:"staff_notes" json:"staff_notes,omitempty"`
	EmploymentType  *string `bun:"employment_type" json:"employment_type,omitempty"`
	WorkTimeModelID *int64  `bun:"work_time_model_id" json:"work_time_model_id,omitempty"`
	// PersonnelNumber is the payroll system's Personalnummer (DATEV LODAS /
	// Lohn und Gehalt match employees by it, #1417). Unique per tenant via
	// partial index; NULL means "not assigned yet". School-scoped like the
	// row itself — a person at two schools of one Träger should carry the
	// same value in both rows, which RLS cannot enforce (documented gap).
	// Generic Staff JSON must not expose it; payroll handlers use a dedicated,
	// permission-gated response.
	PersonnelNumber    *string        `bun:"personnel_number" json:"-"`
	RotationAnchorDate *calendar.Date `bun:"rotation_anchor_date,type:date" json:"rotation_anchor_date,omitempty"`
	// BirthdayDisplayOptOut removes this person from the dashboard birthday
	// display (#1542). Self-service: the staff member sets it on their own
	// profile, so the school-wide setting decides whether staff birthdays are
	// shown at all and this flag lets an individual step out of it.
	BirthdayDisplayOptOut bool       `bun:"birthday_display_opt_out,notnull" json:"birthday_display_opt_out"`
	DeletedAt             *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"-"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
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
