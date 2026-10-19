package peopledirectory

import (
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

func (s *StudentState) Validate() error {
	if s.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if s.SchoolClass == "" {
		return errors.New("school class is required")
	}

	s.SchoolClass = strings.TrimSpace(s.SchoolClass)

	recordTrimPtrStringOrNil(&s.AddressStreet)
	recordTrimPtrStringOrNil(&s.AddressCity)
	recordTrimPtrStringOrNil(&s.AddressPostalCode)

	if err := s.DepartureDays.Validate(); err != nil {
		return err
	}

	if err := s.AllowedDepartureModes.Validate(); err != nil {
		return err
	}

	if err := s.BusDays.Validate(); err != nil {
		return err
	}

	if err := s.PickupDays.Validate(); err != nil {
		return err
	}

	note, err := departure.NormalizeCompanionNote(s.DepartureDays, s.AllowedDepartureModes, s.DepartureCompanionNote, s.DepartureCompanionDays)
	s.DepartureCompanionNote = note
	if err != nil {
		return err
	}

	return nil
}

func (s *StudentState) MarkDepartureCompanionDays(days ...string) {
	if len(days) == 0 {
		return
	}
	if s.DepartureCompanionDays == nil {
		s.DepartureCompanionDays = make(map[string]bool, len(days))
	}
	for _, day := range days {
		s.DepartureCompanionDays[day] = true
	}
}

func recordTrimPtrStringOrNil(sp **string) {
	if *sp == nil || **sp == "" {
		return
	}
	trimmed := strings.TrimSpace(**sp)
	if trimmed == "" {
		*sp = nil
	} else {
		**sp = trimmed
	}
}

func (s *StudentState) SetPerson(person *DirectoryPersonSnapshot) {
	s.Person = person
	if person != nil {
		s.PersonID = person.ID
	}
}
