package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) CreateGuardianContact(ctx context.Context, record peopledirectory.GuardianContactRecord) (int64, error) {
	id, err := s.guardianRows.CreateContact(ctx, toDomainGuardianContact(record))
	return id, mapGuardianPortalError(err)
}

func (s *Service) UpdateGuardianContact(ctx context.Context, guardianID int64, record peopledirectory.GuardianContactRecord) error {
	if guardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}

	return mapGuardianPortalError(s.guardianRows.UpdateContact(ctx, guardianID, toDomainGuardianContact(record)))
}

func (s *Service) ReplaceGuardianPhones(ctx context.Context, guardianID int64, phones []peopledirectory.GuardianPhoneRecord) error {
	if guardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}

	records := make([]domain.GuardianPhoneRecord, 0, len(phones))
	for _, phone := range phones {
		records = append(records, domain.GuardianPhoneRecord(phone))
	}
	return mapGuardianPortalError(s.guardianRows.ReplacePhones(ctx, guardianID, records))
}

func (s *Service) AddGuardianPhoneRecord(ctx context.Context, guardianID int64, phone peopledirectory.GuardianPhoneRecord) (int64, error) {
	if guardianID <= 0 {
		return 0, invalidGuardian("guardian ID is required")
	}

	id, err := s.guardianRows.AddPhone(ctx, guardianID, domain.GuardianPhoneRecord(phone))
	return id, mapGuardianPortalError(err)
}

func (s *Service) SetGuardianPhoneNumber(ctx context.Context, phoneID int64, number string) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}

	return mapGuardianPortalError(s.guardianRows.SetPhoneNumber(ctx, phoneID, number))
}

func (s *Service) DeleteGuardianPhoneRecord(ctx context.Context, phoneID int64) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}

	return mapGuardianPortalError(s.guardianRows.DeletePhone(ctx, phoneID))
}

func (s *Service) LinkGuardianContact(ctx context.Context, link peopledirectory.GuardianContactLink) (int64, bool, error) {
	if link.StudentID <= 0 || link.GuardianProfileID <= 0 {
		return 0, false, invalidGuardian("student ID and guardian ID are required")
	}

	linkID, err := s.guardianRows.LinkContact(ctx, domain.GuardianLinkRecord(link))
	return linkID, linkID > 0, mapGuardianPortalError(err)
}

func (s *Service) PatchGuardianLinkPickup(ctx context.Context, patch peopledirectory.GuardianLinkPickupPatch) (int64, error) {
	if patch.LinkID <= 0 {
		return 0, invalidGuardian("relationship ID is required")
	}
	if patch.CanPickup == nil && patch.IsEmergencyContact == nil && !patch.SetPickupNotes {
		return 0, invalidGuardian("no pickup column supplied")
	}

	affected, err := s.guardianRows.PatchLinkPickup(ctx, domain.GuardianLinkPickupPatch(patch))
	return affected, mapGuardianPortalError(err)
}

func (s *Service) SetGuardianPortalLocale(ctx context.Context, accountID int64, locale string) (int64, error) {
	if accountID <= 0 {
		return 0, invalidGuardian("account ID is required")
	}

	updated, err := s.guardianRows.SetPortalLocale(ctx, accountID, locale)
	return updated, mapGuardianPortalError(err)
}

func (s *Service) SetStudentPhotoConsent(ctx context.Context, state peopledirectory.StudentPhotoState) error {
	if state.StudentID <= 0 {
		return invalidStudent("student ID is required")
	}
	if (state.PhotoConsentGivenAt == nil) != (state.PhotoConsentGivenBy == nil) {
		return invalidStudent("photo consent time and actor must be set together")
	}

	return mapGuardianPortalError(s.studentPhotos.SetPortalConsent(ctx, domain.StudentPhoto{
		StudentID: state.StudentID, PhotoPath: state.PhotoPath,
		PhotoConsentGivenAt: state.PhotoConsentGivenAt, PhotoConsentGivenBy: state.PhotoConsentGivenBy,
	}))
}
