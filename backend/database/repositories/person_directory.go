package repositories

import (
	"context"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// This file is the translation seam of the person write path (#3349): People
// Directory owns users.persons, and the retained person service still carries
// users.Person rows. The seam carries values; the rules — name trimming, the
// tag and account conflicts, the soft delete — are the owner's.

// PersonDirectoryCapability is the owner surface this seam translates to.
type PersonDirectoryCapability interface {
	CreatePerson(context.Context, peopleModule.CreatePerson) (peopleModule.Person, error)
	UpdatePerson(context.Context, peopleModule.UpdatePerson) (peopleModule.Person, error)
	DeletePerson(context.Context, int64) error
}

// PersonDirectory adapts the owner's person writes to the retained model-typed
// contract (services/users.PersonWriter, satisfied structurally so this seam
// does not depend on it).
type PersonDirectory struct{ directory PersonDirectoryCapability }

func NewPersonDirectory(directory PersonDirectoryCapability) *PersonDirectory {
	return &PersonDirectory{directory: directory}
}

// CreatePerson writes the row and reflects the stored values — the owner
// assigns the id and the timestamps, and trims the names — back onto the
// caller's struct, which is the contract the retained handlers rely on.
func (d *PersonDirectory) CreatePerson(ctx context.Context, person *userModels.Person) error {
	created, err := d.directory.CreatePerson(ctx, peopleModule.CreatePerson{
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Birthday:  personBirthday(person),
		TagID:     person.TagID,
		AccountID: person.AccountID,
	})
	if err != nil {
		return translatePersonDirectoryError(err)
	}
	applyPersonRecord(person, created)
	return nil
}

func (d *PersonDirectory) UpdatePerson(ctx context.Context, person *userModels.Person) error {
	updated, err := d.directory.UpdatePerson(ctx, peopleModule.UpdatePerson{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Birthday:  personBirthday(person),
		TagID:     person.TagID,
		AccountID: person.AccountID,
	})
	if err != nil {
		return translatePersonDirectoryError(err)
	}
	applyPersonRecord(person, updated)
	return nil
}

func (d *PersonDirectory) DeletePerson(ctx context.Context, id int64) error {
	return translatePersonDirectoryError(d.directory.DeletePerson(ctx, id))
}

func personBirthday(person *userModels.Person) string {
	if person.Birthday == nil {
		return ""
	}
	return person.Birthday.String()
}

func applyPersonRecord(person *userModels.Person, record peopleModule.Person) {
	person.ID, person.CreatedAt, person.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	person.SetTenantID(record.TenantID)
	person.FirstName, person.LastName = record.FirstName, record.LastName
	person.TagID, person.AccountID = record.TagID, record.AccountID
	person.Birthday = userModels.OptionalCalendarDate(record.Birthday)
	person.DeletedAt = record.DeletedAt
}

// translatePersonDirectoryError restates the owner's missing person in the
// vocabulary the retained person service branches on. Everything else — the
// tag and account conflicts, a validation refusal — passes through unchanged:
// the handlers above already render an unrecognized write failure the same way
// they rendered the constraint violation it replaces.
func translatePersonDirectoryError(err error) error {
	if err != nil && errors.Is(err, peopleModule.ErrPersonNotFound) {
		return userModels.ErrPersonRowMissing
	}
	return err
}
