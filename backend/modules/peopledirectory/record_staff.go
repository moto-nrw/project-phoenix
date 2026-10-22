package peopledirectory

import (
	"errors"
)

func (s *Staff) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if s.EmploymentType != nil {
		switch *s.EmploymentType {
		case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeMinijob:

		default:
			return errors.New("employment_type must be 'full_time', 'part_time', or 'minijob'")
		}
	}

	return nil
}
