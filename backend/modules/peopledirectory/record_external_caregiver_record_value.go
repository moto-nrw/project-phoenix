package peopledirectory

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ExternalCaregiverRecord makes a staff member an external caregiver (ADR 0089).
type ExternalCaregiverRecord struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	TenantID          int64          `json:"tenant_id"`
	StaffID           int64          `json:"staff_id"`
	Organization      string         `json:"organization,omitempty"`
	ContactEmail      string         `json:"contact_email,omitempty"`
	ContactPhone      string         `json:"contact_phone,omitempty"`
	ActivityExpertise string         `json:"activity_expertise"`
	StartDate         *calendar.Date `json:"start_date,omitempty"`
	EndDate           *calendar.Date `json:"end_date,omitempty"`
	Notes             string         `json:"notes,omitempty"`

	// Relations not stored in the database
	Staff *Staff `json:"staff,omitempty"`
}
