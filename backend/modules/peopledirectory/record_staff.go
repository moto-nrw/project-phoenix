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

func (s *Staff) IsMinijob() bool {
	return s.EmploymentType != nil && *s.EmploymentType == EmploymentTypeMinijob
}

func (s *Staff) SetPerson(person *DirectoryPersonSnapshot) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}

func (s *Staff) GetFullName() string {
	if s.Person != nil {
		return s.Person.GetFullName()
	}
	return ""
}

func (s *Staff) AddNotes(notes string) {
	if s.StaffNotes == "" {
		s.StaffNotes = notes
	} else {
		s.StaffNotes += "\n" + notes
	}
}
