package users

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Guest represents a guest instructor in the system
type Guest struct {
	base.Model `bun:"schema:users,table:guests"`
	base.TenantModel
	StaffID           int64          `bun:"staff_id,notnull" json:"staff_id"`
	Organization      string         `bun:"organization" json:"organization,omitempty"`
	ContactEmail      string         `bun:"contact_email" json:"contact_email,omitempty"`
	ContactPhone      string         `bun:"contact_phone" json:"contact_phone,omitempty"`
	ActivityExpertise string         `bun:"activity_expertise,notnull" json:"activity_expertise"`
	StartDate         *timezone.Date `bun:"start_date" json:"start_date,omitempty"`
	EndDate           *timezone.Date `bun:"end_date" json:"end_date,omitempty"`
	Notes             string         `bun:"notes" json:"notes,omitempty"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
}

// Validate ensures guest data is valid
func (g *Guest) Validate() error {
	if g.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if g.ActivityExpertise == "" {
		return errors.New("activity expertise is required")
	}

	// Trim spaces from fields
	g.ActivityExpertise = strings.TrimSpace(g.ActivityExpertise)

	if g.Organization != "" {
		g.Organization = strings.TrimSpace(g.Organization)
	}

	// Validate contact email if provided
	if g.ContactEmail != "" {
		g.ContactEmail = strings.TrimSpace(g.ContactEmail)
		if _, err := mail.ParseAddress(g.ContactEmail); err != nil {
			return errors.New("invalid contact email format")
		}
	}

	// Validate contact phone if provided. Uses the shared canonical
	// validator (phone_validation.go) so this rule can never drift from
	// the student/enrollment phone format.
	if g.ContactPhone != "" {
		g.ContactPhone = strings.TrimSpace(g.ContactPhone)
		if err := ValidateOptionalPhone(g.ContactPhone); err != nil {
			return errors.New("invalid contact phone format")
		}
	}

	// Validate date range if both dates are provided
	if g.StartDate != nil && g.EndDate != nil {
		if g.EndDate.Before(*g.StartDate) {
			return errors.New("end date cannot be before start date")
		}
	}

	return nil
}

// SetStaff links this guest to a staff member
func (g *Guest) SetStaff(staff *Staff) {
	g.Staff = staff
	if staff != nil {
		g.StaffID = staff.ID
	}
}

// GetFullName returns the full name of the guest from the linked staff and person
func (g *Guest) GetFullName() string {
	if g.Staff != nil && g.Staff.Person != nil {
		return g.Staff.Person.GetFullName()
	}
	return ""
}

// AddNotes adds notes about this guest
func (g *Guest) AddNotes(notes string) {
	if g.Notes == "" {
		g.Notes = notes
	} else {
		g.Notes += "\n" + notes
	}
}
