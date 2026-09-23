package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e engine) CreateGuardianContact(ctx context.Context, record peopledirectory.GuardianContactRecord) (int64, error) {
	id, err := e.guardians.CreateContact(ctx, toDomainGuardianContact(record))
	return id, mapGuardianPortalError(err)
}

func (e engine) UpdateGuardianContact(ctx context.Context, guardianID int64, record peopledirectory.GuardianContactRecord) error {
	return mapGuardianPortalError(e.guardians.UpdateContact(ctx, guardianID, toDomainGuardianContact(record)))
}

func (e engine) ReplaceGuardianPhones(ctx context.Context, guardianID int64, phones []peopledirectory.GuardianPhoneRecord) error {
	records := make([]domain.GuardianPhoneRecord, 0, len(phones))
	for _, phone := range phones {
		records = append(records, domain.GuardianPhoneRecord(phone))
	}
	return mapGuardianPortalError(e.guardians.ReplacePhones(ctx, guardianID, records))
}

func (e engine) AddGuardianPhoneRecord(ctx context.Context, guardianID int64, phone peopledirectory.GuardianPhoneRecord) (int64, error) {
	id, err := e.guardians.AddPhone(ctx, guardianID, domain.GuardianPhoneRecord(phone))
	return id, mapGuardianPortalError(err)
}

func (e engine) SetGuardianPhoneNumber(ctx context.Context, phoneID int64, number string) error {
	return mapGuardianPortalError(e.guardians.SetPhoneNumber(ctx, phoneID, number))
}

func (e engine) DeleteGuardianPhoneRecord(ctx context.Context, phoneID int64) error {
	return mapGuardianPortalError(e.guardians.DeletePhone(ctx, phoneID))
}

func (e engine) LinkGuardianContact(ctx context.Context, link peopledirectory.GuardianContactLink) (int64, bool, error) {
	linkID, err := e.guardians.LinkContact(ctx, domain.GuardianLinkRecord(link))
	return linkID, linkID > 0, mapGuardianPortalError(err)
}

func (e engine) PatchGuardianLinkPickup(ctx context.Context, patch peopledirectory.GuardianLinkPickupPatch) (int64, error) {
	affected, err := e.guardians.PatchLinkPickup(ctx, domain.GuardianLinkPickupPatch(patch))
	return affected, mapGuardianPortalError(err)
}

func (e engine) SetGuardianPortalLocale(ctx context.Context, accountID int64, locale string) (int64, error) {
	updated, err := e.guardians.SetPortalLocale(ctx, accountID, locale)
	return updated, mapGuardianPortalError(err)
}

func (e engine) SetStudentPhotoConsent(ctx context.Context, state peopledirectory.StudentPhotoState) error {
	return mapGuardianPortalError(e.studentPhotos.SetPortalConsent(ctx, domain.StudentPhoto{
		StudentID: state.StudentID, PhotoPath: state.PhotoPath,
		PhotoConsentGivenAt: state.PhotoConsentGivenAt, PhotoConsentGivenBy: state.PhotoConsentGivenBy,
	}))
}

func toDomainGuardianContact(record peopledirectory.GuardianContactRecord) domain.GuardianContact {
	return domain.GuardianContact{
		FirstName: record.FirstName, LastName: record.LastName, Email: record.Email,
		AddressStreet: record.AddressStreet, AddressCity: record.AddressCity, PostalCode: record.AddressPostalCode,
		PreferredContactMethod: record.PreferredContactMethod, LanguagePreference: record.LanguagePreference,
	}
}

// mapGuardianPortalError translates the portal commands' domain outcomes;
// the e-mail conflict keeps the database error in its chain.
func mapGuardianPortalError(err error) error {
	var invalid *domain.GuardianInvalidError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &invalid):
		return &peopledirectory.InvalidGuardianError{Reason: invalid.Reason}
	case errors.Is(err, domain.ErrGuardianEmailTaken):
		return fmt.Errorf("%w: %w", peopledirectory.ErrGuardianEmailTaken, err)
	case errors.Is(err, domain.ErrGuardianNotFound):
		return peopledirectory.ErrGuardianNotFound
	case errors.Is(err, domain.ErrGuardianPhoneNotFound):
		return peopledirectory.ErrGuardianPhoneNotFound
	default:
		return mapError(err)
	}
}
