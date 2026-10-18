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

func (g *GuardianProfileSnapshot) GetFullName() string {
	first := strings.TrimSpace(g.FirstName)
	last := strings.TrimSpace(g.LastName)
	if first == "" && last == "" {
		return ""
	}
	if first == "" {
		return last
	}
	if last == "" {
		return first
	}
	return first + " " + last
}

func (g *GuardianProfileSnapshot) GetPreferredContact() string {

	if contact := g.getContactByMethod(g.PreferredContactMethod); contact != "" {
		return contact
	}

	if primary := g.GetPrimaryPhone(); primary != "" {
		return primary
	}
	return recordDeref(g.Email)
}

func (g *GuardianProfileSnapshot) GetPrimaryPhone() string {
	if len(g.PhoneNumbers) == 0 {
		return ""
	}

	for _, p := range g.PhoneNumbers {
		if p.IsPrimary {
			return p.PhoneNumber
		}
	}

	return g.PhoneNumbers[0].PhoneNumber
}

func (g *GuardianProfileSnapshot) GetPhoneByType(phoneType string) string {
	for _, p := range g.PhoneNumbers {
		if p.PhoneType == phoneType {
			return p.PhoneNumber
		}
	}
	return ""
}

func (g *GuardianProfileSnapshot) getContactByMethod(method string) string {
	switch method {
	case "email":
		return recordDeref(g.Email)
	case "mobile", "sms":
		return g.GetPhoneByType(PhoneTypeMobile)
	case "phone":
		return g.GetPhoneByType(PhoneTypeHome)
	default:
		return ""
	}
}

func (g *GuardianProfileSnapshot) CanInvite() bool {
	return g.Email != nil && *g.Email != "" && !g.HasAccount
}

func (g *GuardianProfileSnapshot) HasEmail() bool {
	return g.Email != nil && *g.Email != ""
}

func (g *GuardianProfileSnapshot) HasPortalAccount() bool {
	return g.HasAccount || g.AccountID != nil
}
