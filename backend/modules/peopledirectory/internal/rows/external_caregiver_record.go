package users

import (
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ExternalCaregiverRecord is one users.external_caregivers row (ADR 0090).
type ExternalCaregiverRecord struct {
	Model `bun:"schema:users,table:external_caregivers"`
	TenantModel
	StaffID           int64          `bun:"staff_id,notnull" json:"staff_id"`
	Organization      string         `bun:"organization" json:"organization,omitempty"`
	ContactEmail      string         `bun:"contact_email" json:"contact_email,omitempty"`
	ContactPhone      string         `bun:"contact_phone" json:"contact_phone,omitempty"`
	ActivityExpertise string         `bun:"activity_expertise,notnull" json:"activity_expertise"`
	StartDate         *calendar.Date `bun:"start_date" json:"start_date,omitempty"`
	EndDate           *calendar.Date `bun:"end_date" json:"end_date,omitempty"`
	Notes             string         `bun:"notes" json:"notes,omitempty"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
}
