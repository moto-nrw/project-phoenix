package enrollment

import (
	"errors"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// Validate enforces guardian_profiles.CHECK (email OR ≥1 phone) plus
// per-phone validity. Relationship type is admitted as free text;
// the decision service maps it onto the enum via the same helper the
// CSV importer uses.
func (c *ContactEntry) Validate() error {
	c.FirstName = textnorm.Name(c.FirstName)
	c.LastName = textnorm.Name(c.LastName)
	if c.FirstName == "" || c.LastName == "" {
		return errors.New("contact first_name and last_name are required")
	}
	c.Email = strings.TrimSpace(c.Email)
	hasPhone := false
	for i := range c.PhoneNumbers {
		if err := c.PhoneNumbers[i].Validate(); err != nil {
			return fmt.Errorf("contact %s %s: %w", c.FirstName, c.LastName, err)
		}
		hasPhone = true
	}
	if c.Email == "" && !hasPhone {
		return errors.New("contact requires at least one of email or phone_numbers")
	}
	if c.EmergencyPriority < 0 {
		return errors.New("emergency_priority must be non-negative")
	}
	return nil
}
