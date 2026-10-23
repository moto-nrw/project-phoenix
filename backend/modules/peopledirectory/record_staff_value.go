package peopledirectory

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Staff struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	TenantID        int64   `json:"tenant_id"`
	PersonID        int64   `json:"person_id"`
	StaffNotes      string  `json:"staff_notes,omitempty"`
	EmploymentType  *string `json:"employment_type,omitempty"`
	WorkTimeModelID *int64  `json:"work_time_model_id,omitempty"`
	// PersonnelNumber is the payroll system's Personalnummer (DATEV LODAS /
	// Lohn und Gehalt match employees by it, #1417). Unique per tenant via
	// partial index; NULL means "not assigned yet". School-scoped like the
	// row itself — a person at two schools of one Träger should carry the
	// same value in both rows, which RLS cannot enforce (documented gap).
	// Generic Staff JSON must not expose it; payroll handlers use a dedicated,
	// permission-gated response.
	PersonnelNumber    *string        `json:"-"`
	RotationAnchorDate *calendar.Date `json:"rotation_anchor_date,omitempty"`
	// BirthdayDisplayOptOut removes this person from the dashboard birthday
	// display (#1542). Self-service: the staff member sets it on their own
	// profile, so the school-wide setting decides whether staff birthdays are
	// shown at all and this flag lets an individual step out of it.
	BirthdayDisplayOptOut bool       `json:"birthday_display_opt_out"`
	DeletedAt             *time.Time `json:"-"`
	// HasExternalCaregiverRecord is School Membership's raw marker: the staff
	// member has an active external caregiver record (#3823, ADR 0083). Every
	// read fills it.
	HasExternalCaregiverRecord bool `json:"-"`
	// IsExternalCaregiver applies School Membership's rule
	// (schoolmembership.ExternalCaregiver): an active record and a known
	// person, with or without a moto account (#4573). Only the reads that
	// attach the person (ListAllWithPerson, FindWithPersonByIDs) set it;
	// every other read leaves it false. Time tracking and the duty roster
	// skip these people.
	IsExternalCaregiver bool `json:"-"`

	// Relations
	Person *DirectoryPersonSnapshot `json:"person,omitempty"`
}
