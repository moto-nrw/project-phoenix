package application

import (
	"context"
	"errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// PersonRows maps persisted person values at the retained directory boundary.
// It calls the native service instead of looping through this owner's facade.
type PersonRows struct{ directory *Service }

func NewPersonRows(directory *Service) *PersonRows { return &PersonRows{directory: directory} }

// CreatePerson writes the row and reflects the stored values — the owner
// assigns the id and the timestamps, and trims the names — back onto the
// caller's struct, which is the contract the retained handlers rely on.
func (d *PersonRows) CreatePerson(ctx context.Context, person *peopleModule.DirectoryPersonSnapshot) error {
	input, err := peopleModule.NormalizePersonCreate(peopleModule.CreatePerson{
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Birthday:  personBirthday(person),
		TagID:     person.TagID,
		AccountID: person.AccountID,
	})
	if err != nil {
		return translatePersonWriteError(MapError(err))
	}
	created, err := d.directory.Create(ctx, peopleModule.CreatePerson(input))
	if err != nil {
		return translatePersonWriteError(MapError(err))
	}
	applyPersonRecord(person, created)
	return nil
}

func (d *PersonRows) UpdatePerson(ctx context.Context, person *peopleModule.DirectoryPersonSnapshot) error {
	input, err := peopleModule.NormalizePersonUpdate(peopleModule.UpdatePerson{
		ID:        person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		Birthday:  personBirthday(person),
		TagID:     person.TagID,
		AccountID: person.AccountID,
	})
	if err != nil {
		return translatePersonWriteError(MapError(err))
	}
	updated, err := d.directory.UpdatePerson(ctx, peopleModule.UpdatePerson(input))
	if err != nil {
		return translatePersonWriteError(MapError(err))
	}
	applyPersonRecord(person, updated)
	return nil
}

func (d *PersonRows) DeletePerson(ctx context.Context, id int64) error {
	if id <= 0 {
		return &peopleModule.InvalidPersonError{Reason: "person ID is required"}
	}
	return translatePersonWriteError(MapError(d.directory.DeletePerson(ctx, id)))
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
