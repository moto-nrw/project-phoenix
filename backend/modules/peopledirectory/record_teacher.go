package peopledirectory

import (
	"errors"
	"strings"
)

func (t *Teacher) Validate() error {
	if t.StaffID <= 0 {
		return errors.New("staff ID is required")
	}

	t.Specialization = strings.TrimSpace(t.Specialization)

	if t.Role != "" {
		t.Role = strings.TrimSpace(t.Role)
	}

	return nil
}
