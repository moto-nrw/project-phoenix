package compose

import (
	"context"
	"errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// This file is the translation seam of the person write path (#3349): People
// Directory owns users.persons, and the retained person service still carries
// users.Person rows. The seam carries values; the rules — name trimming, the
// tag and account conflicts, the soft delete — are the owner's.

// PersonWriteCapability is the owner surface this seam translates to.
type PersonWriteCapability interface {
	CreatePerson(context.Context, peopleModule.CreatePerson) (peopleModule.Person, error)
	UpdatePerson(context.Context, peopleModule.UpdatePerson) (peopleModule.Person, error)
	DeletePerson(context.Context, int64) error
}

// PersonWrites adapts the owner's person writes to the retained model-typed
// contract (PersonWriter, satisfied structurally).
type PersonWrites struct{ directory PersonWriteCapability }

// NewPersonWrites binds the owner's person writes behind the retained
// PersonWriter contract.
func NewPersonWrites(directory PersonWriteCapability) *PersonWrites {
	return &PersonWrites{directory: directory}
}

// CreatePerson writes the row and reflects the stored values — the owner
// assigns the id and the timestamps, and trims the names — back onto the
// caller's struct, which is the contract the retained handlers rely on.
func (d *PersonWrites) CreatePerson(ctx context.Context, person *peopleModule.DirectoryPersonSnapshot) error {
	created, err := d.directory.CreatePerson(ctx, peopleModule.CreatePerson{
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Birthday:  personBirthday(person),
		TagID:     person.TagID,
		AccountID: person.AccountID,
	})
	if err != nil {
		return translatePersonWriteError(err)
	}
	applyPersonRecord(person, created)
	return nil
}

func (d *PersonWrites) UpdatePerson(ctx context.Context, person *peopleModule.DirectoryPersonSnapshot) error {
	updated, err := d.directory.UpdatePerson(ctx, peopleModule.UpdatePerson{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Birthday:  personBirthday(person),
		TagID:     person.TagID,
		AccountID: person.AccountID,
	})
	if err != nil {
		return translatePersonWriteError(err)
	}
	applyPersonRecord(person, updated)
	return nil
}

func (d *PersonWrites) DeletePerson(ctx context.Context, id int64) error {
	return translatePersonWriteError(d.directory.DeletePerson(ctx, id))
}

func personBirthday(person *peopleModule.DirectoryPersonSnapshot) string {
	if person.Birthday == nil {
		return ""
	}
	return person.Birthday.String()
}

func applyPersonRecord(person *peopleModule.DirectoryPersonSnapshot, record peopleModule.Person) {
	person.ID, person.CreatedAt, person.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	person.SetTenantID(record.TenantID)
	person.FirstName, person.LastName = record.FirstName, record.LastName
	person.TagID, person.AccountID = record.TagID, record.AccountID
	person.Birthday = peopleModule.OptionalCalendarDate(record.Birthday)
	person.DeletedAt = record.DeletedAt
}

// translatePersonWriteError restates the owner's missing person in the
// vocabulary the retained person service branches on. Everything else — the
// tag and account conflicts, a validation refusal — passes through unchanged:
// the handlers above already render an unrecognized write failure the same way
// they rendered the constraint violation it replaces.
func translatePersonWriteError(err error) error {
	if err != nil && errors.Is(err, peopleModule.ErrPersonNotFound) {
		return peopleModule.ErrPersonRowMissing
	}
	return err
}
