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

func (t *Teacher) SetStaff(staff *Staff) {
	t.Staff = staff
	if staff != nil {
		t.StaffID = staff.ID
	}
}

func (t *Teacher) GetFullName() string {
	if t.Staff != nil && t.Staff.Person != nil {
		return t.Staff.Person.GetFullName()
	}
	return ""
}

func (t *Teacher) GetTitle() string {
	if t.Role != "" {
		return t.Role
	}
	return t.Specialization
}

func (t *Teacher) HasQualifications() bool {
	return t.Qualifications != ""
}
