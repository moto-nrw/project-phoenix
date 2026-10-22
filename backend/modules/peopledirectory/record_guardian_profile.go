package peopledirectory

import (
	"errors"
	"net/mail"
	"strings"
)

func (g *GuardianProfileSnapshot) Validate() error {

	g.FirstName = strings.TrimSpace(g.FirstName)
	g.LastName = strings.TrimSpace(g.LastName)

	if g.Email != nil && *g.Email != "" {
		*g.Email = strings.TrimSpace(strings.ToLower(*g.Email))
		if _, err := mail.ParseAddress(*g.Email); err != nil {
			return errors.New("invalid email format")
		}
	}

	validMethods := map[string]bool{
		"email":  true,
		"phone":  true,
		"mobile": true,
		"sms":    true,
	}
	if g.PreferredContactMethod != "" && !validMethods[g.PreferredContactMethod] {
		return errors.New("invalid preferred contact method")
	}

	return nil
}
