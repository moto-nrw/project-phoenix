package active

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// SessionStaff is membership information attached to a supervision read.
type SessionStaff struct {
	ID                    int64               `json:"id"`
	TenantID              int64               `json:"tenant_id"`
	CreatedAt             time.Time           `json:"created_at"`
	UpdatedAt             time.Time           `json:"updated_at"`
	PersonID              int64               `json:"person_id"`
	StaffNotes            string              `json:"staff_notes,omitempty"`
	EmploymentType        *string             `json:"employment_type,omitempty"`
	WorkTimeModelID       *int64              `json:"work_time_model_id,omitempty"`
	RotationAnchorDate    *timezone.Date      `json:"rotation_anchor_date,omitempty"`
	BirthdayDisplayOptOut bool                `json:"birthday_display_opt_out"`
	Person                *SessionStaffPerson `json:"person,omitempty"`
}

// SessionStaffPerson preserves the person facts returned by the owner directory.
type SessionStaffPerson struct {
	ID        int64          `json:"id"`
	TenantID  int64          `json:"tenant_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Birthday  *timezone.Date `json:"birthday,omitempty"`
	TagID     *string        `json:"tag_id,omitempty"`
	AccountID *int64         `json:"account_id,omitempty"`
}

func (p *SessionStaffPerson) GetFullName() string { return p.FirstName + " " + p.LastName }
