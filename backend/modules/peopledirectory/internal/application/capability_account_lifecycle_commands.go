package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) CreateSchoolIdentityPerson(ctx context.Context, person peopledirectory.SchoolIdentityPerson) (int64, error) {
	firstName, lastName := peopledirectory.NormalizePersonNames(person.FirstName, person.LastName)
	if firstName == "" || lastName == "" {
		return 0, invalid("first name and last name are required")
	}
	if err := peopledirectory.ValidatePersonReferences(person.TagID, nil); err != nil {
		return 0, err
	}
	created, err := s.Create(ctx, peopledirectory.CreatePerson{FirstName: firstName, LastName: lastName, TagID: person.TagID})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (s *Service) CreateInvitedGuardian(ctx context.Context, guardian peopledirectory.InvitedGuardian) (int64, error) {
	email := guardian.Email
	return s.CreateGuardianContact(ctx, peopledirectory.GuardianContactRecord{
		FirstName: guardian.FirstName, LastName: guardian.LastName, Email: &email,
		PreferredContactMethod: invitedGuardianContactMethod, LanguagePreference: invitedGuardianLanguage,
	})
}

func (s *Service) CreateAccountPerson(ctx context.Context, accountID int64, firstName, lastName string) error {
	_, err := s.createAsGiven(ctx, peopledirectory.CreatePerson{FirstName: firstName, LastName: lastName, AccountID: &accountID})
	return err
}

func (s *Service) CreateChildPerson(ctx context.Context, child peopledirectory.ChildPerson) (peopledirectory.Person, error) {
	return s.createAsGiven(ctx, peopledirectory.CreatePerson{FirstName: child.FirstName, LastName: child.LastName, Birthday: child.Birthday})
}

func (s *Service) createAsGiven(ctx context.Context, person peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	if person.FirstName == "" || person.LastName == "" {
		return peopledirectory.Person{}, invalid("first name and last name are required")
	}
	person.FirstName, person.LastName = strings.TrimSpace(person.FirstName), strings.TrimSpace(person.LastName)

	value, err := s.Create(ctx, peopledirectory.CreatePerson{
		FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday,
		TagID: person.TagID, AccountID: person.AccountID,
	})
	return value, MapError(err)
}

func (s *Service) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	if firstName != nil && *firstName == "" || lastName != nil && *lastName == "" {
		return invalid("first name and last name are required")
	}
	return (PersonWrites{persons: s}).RenamePerson(ctx, personID, firstName, lastName)
}

func (s *Service) SetPersonDetails(ctx context.Context, personID int64, details peopledirectory.PersonDetails) error {
	if details.FirstName == "" || details.LastName == "" {
		return invalid("first name and last name are required")
	}
	details.FirstName, details.LastName = strings.TrimSpace(details.FirstName), strings.TrimSpace(details.LastName)
	return (PersonWrites{persons: s}).SetPersonDetails(ctx, personID, details)
}

func (s *Service) AnonymizeAndSoftDeletePerson(ctx context.Context, personID int64) error {
	if personID <= 0 {
		return invalid("person ID is required")
	}
	return (PersonWrites{persons: s}).AnonymizeAndSoftDeletePerson(ctx, personID)
}

// Invited guardians are reached by e-mail and spoken to in German until they
// choose otherwise: the contact a guardian invitation creates carries these
// preferences.
const (
	invitedGuardianContactMethod = "email"
	invitedGuardianLanguage      = "de"
)
