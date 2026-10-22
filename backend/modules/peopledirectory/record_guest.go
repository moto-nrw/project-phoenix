package peopledirectory

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

func (g *Guest) Validate() error {
	if g.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	if g.ActivityExpertise == "" {
		return errors.New("activity expertise is required")
	}

	g.ActivityExpertise = strings.TrimSpace(g.ActivityExpertise)

	if g.Organization != "" {
		g.Organization = strings.TrimSpace(g.Organization)
	}

	if g.ContactEmail != "" {
		g.ContactEmail = strings.TrimSpace(g.ContactEmail)
		if _, err := mail.ParseAddress(g.ContactEmail); err != nil {
			return errors.New("invalid contact email format")
		}
	}

	if g.ContactPhone != "" {
		g.ContactPhone = strings.TrimSpace(g.ContactPhone)
		if err := contact.ValidateOptionalPhone(g.ContactPhone); err != nil {
			return errors.New("invalid contact phone format")
		}
	}

	if g.StartDate != nil && g.EndDate != nil {
		if g.EndDate.Before(*g.StartDate) {
			return errors.New("end date cannot be before start date")
		}
	}

	return nil
}
