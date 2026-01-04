package users

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const guestTableName = "users.guests"

// Guest represents a guest instructor in the system
type Guest struct {
	base.Model        `bun:"schema:users,table:guests"`
	StaffID           int64      `bun:"staff_id,notnull,unique" json:"staff_id"`
	Organization      string     `bun:"organization" json:"organization,omitempty"`
	ContactEmail      string     `bun:"contact_email" json:"contact_email,omitempty"`
	ContactPhone      string     `bun:"contact_phone" json:"contact_phone,omitempty"`
	ActivityExpertise string     `bun:"activity_expertise,notnull" json:"activity_expertise"`
	StartDate         *time.Time `bun:"start_date" json:"start_date,omitempty"`
	EndDate           *time.Time `bun:"end_date" json:"end_date,omitempty"`
	Notes             string     `bun:"notes" json:"notes,omitempty"`

	// Relations not stored in the database
	Staff *Staff `bun:"-" json:"staff,omitempty"`
}

func (s *Guest) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(guestTableName)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(guestTableName)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(guestTableName)
	}
	return nil
}

// TableName returns the database table name
func (g *Guest) TableName() string {
	return guestTableName
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

	// Validate contact phone if provided
	if g.ContactPhone != "" {
		g.ContactPhone = strings.TrimSpace(g.ContactPhone)
		phonePattern := regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
		if !phonePattern.MatchString(g.ContactPhone) {
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

// IsActive checks if the guest is currently active based on start/end dates
func (g *Guest) IsActive() bool {
	now := time.Now()

	// If no dates are set, consider active
	if g.StartDate == nil && g.EndDate == nil {
		return true
	}

	// If only start date is set, check if it's before now
	if g.StartDate != nil && g.EndDate == nil {
		return !now.Before(*g.StartDate)
	}

	// If only end date is set, check if it's after now
	if g.StartDate == nil && g.EndDate != nil {
		return !now.After(*g.EndDate)
	}

	// Both dates are set, check if now is between them
	return !now.Before(*g.StartDate) && !now.After(*g.EndDate)
}

// AddNotes adds notes about this guest
func (g *Guest) AddNotes(notes string) {
	if g.Notes == "" {
		g.Notes = notes
	} else {
		g.Notes += "\n" + notes
	}
}

// GetID returns the entity's ID
func (m *Guest) GetID() interface{} {
	return m.ID
}

// GetCreatedAt returns the creation timestamp
func (m *Guest) GetCreatedAt() time.Time {
	return m.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (m *Guest) GetUpdatedAt() time.Time {
	return m.UpdatedAt
}
