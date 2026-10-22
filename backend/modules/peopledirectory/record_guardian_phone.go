package peopledirectory

import (
	"errors"
	"regexp"
	"strings"
)

func (g *GuardianProfilePhoneSnapshot) Validate() error {

	if g.GuardianProfileID <= 0 {
		return errors.New("guardian profile ID is required")
	}

	if strings.TrimSpace(g.PhoneNumber) == "" {
		return errors.New("phone number is required")
	}

	g.PhoneNumber = strings.TrimSpace(g.PhoneNumber)

	phoneRegex := regexp.MustCompile(`^[\d\s\+\-\(\)]+$`)
	if !phoneRegex.MatchString(g.PhoneNumber) {
		return errors.New("invalid phone number format")
	}

	digitsOnly := regexp.MustCompile(`\d`).FindAllString(g.PhoneNumber, -1)
	if len(digitsOnly) < recordMinPhoneDigits {
		return errors.New("phone number must contain at least 3 digits")
	}

	if !recordValidPhoneTypes[g.PhoneType] {
		return errors.New("invalid phone type, must be one of: mobile, home, work, other")
	}

	if g.Label != nil {
		trimmed := strings.TrimSpace(*g.Label)
		if trimmed == "" {
			g.Label = nil
		} else {
			g.Label = &trimmed
		}
	}

	if g.Priority < recordMinPhonePriority {
		g.Priority = recordMinPhonePriority
	}

	return nil
}

const recordMinPhoneDigits = 3
const recordMinPhonePriority = 1

var recordValidPhoneTypes = map[string]bool{PhoneTypeMobile: true, PhoneTypeHome: true, PhoneTypeWork: true, PhoneTypeOther: true}
