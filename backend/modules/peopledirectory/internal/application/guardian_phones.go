package application

import (
	"context"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The phone numbers of a guardian profile.

// AddPhoneNumber adds a new phone number to a guardian
func (s *RetainedGuardianService) AddPhoneNumber(ctx context.Context, guardianID int64, req peoplePorts.PhoneNumberCreateRequest) (*peopleValues.GuardianProfilePhoneSnapshot, error) {
	// Lock the guardian profile so phone-list writers serialize with the
	// parents-portal contact path, which locks the same row before its wholesale
	// phone replace (delete-all + re-insert) — see UpdateGuardian. Without this a
	// staff phone add could land between the parent's read-old-phones and its
	// replace, then be wiped by the replace. Held for the request transaction.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}
	// Verify guardian exists
	if _, err := s.GuardianProfileRepo.FindByID(ctx, guardianID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Get current count to determine if this should be primary
	count, err := s.GuardianPhoneNumberRepo.CountByGuardianID(ctx, guardianID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing phone numbers: %w", err)
	}

	phone, err := s.newGuardianPhone(ctx, guardianID, req, count)
	if err != nil {
		return nil, err
	}

	if err := phone.Validate(); err != nil {
		return nil, newGuardianValidationError(err)
	}

	// If setting as primary, unset existing primaries first
	if phone.IsPrimary && count > 0 {
		if err := s.GuardianPhoneNumberRepo.UnsetAllPrimary(ctx, guardianID); err != nil {
			return nil, fmt.Errorf("failed to unset existing primary: %w", err)
		}
	}

	if err := s.GuardianPhoneNumberRepo.Create(ctx, phone); err != nil {
		return nil, fmt.Errorf("failed to create phone number: %w", err)
	}

	return phone, nil
}

// newGuardianPhone builds the phone row AddPhoneNumber inserts: next
// priority, a known phone type, and primary when it is the guardian's first
// number or the request asks for it.
func (s *RetainedGuardianService) newGuardianPhone(ctx context.Context, guardianID int64, req peoplePorts.PhoneNumberCreateRequest, count int) (*peopleValues.GuardianProfilePhoneSnapshot, error) {
	// Get next priority
	priority, err := s.GuardianPhoneNumberRepo.GetNextPriority(ctx, guardianID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next priority: %w", err)
	}

	// Determine phone type
	phoneType := string(req.PhoneType)
	if !peopleValues.ValidPhoneType(phoneType) {
		phoneType = peopleValues.PhoneTypeMobile // Default to mobile
	}

	phone := &peopleValues.GuardianProfilePhoneSnapshot{
		GuardianProfileID: guardianID,
		PhoneNumber:       req.PhoneNumber,
		PhoneType:         phoneType,
		Label:             req.Label,
		// If this is the first phone number or explicitly set as primary, make it primary
		IsPrimary: req.IsPrimary || count == 0,
		Priority:  priority,
	}
	phone.SetTenantID(s.transactions.TenantID(ctx))
	return phone, nil
}

// UpdatePhoneNumber updates an existing phone number
func (s *RetainedGuardianService) UpdatePhoneNumber(ctx context.Context, phoneID int64, req peoplePorts.PhoneNumberUpdateRequest) error {
	phone, err := s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}
	// Lock the owning guardian profile so this edit serializes with the
	// parents-portal contact path's wholesale phone replace (see UpdateGuardian).
	// The phone→profile association is immutable, so reading the phone first and
	// then locking its profile is safe.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, phone.GuardianProfileID); err != nil {
		return fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Update fields if provided
	if req.PhoneNumber != nil {
		phone.PhoneNumber = *req.PhoneNumber
	}
	if req.PhoneType != nil {
		phoneType := string(*req.PhoneType)
		if peopleValues.ValidPhoneType(phoneType) {
			phone.PhoneType = phoneType
		}
	}
	if req.Label != nil {
		phone.Label = req.Label
	}
	if req.Priority != nil {
		phone.Priority = *req.Priority
	}

	if err := phone.Validate(); err != nil {
		return newGuardianValidationError(err)
	}

	// Handle primary flag change
	if req.IsPrimary != nil && *req.IsPrimary && !phone.IsPrimary {
		phone.IsPrimary = true
		if err := s.GuardianPhoneNumberRepo.UnsetAllPrimary(ctx, phone.GuardianProfileID); err != nil {
			return fmt.Errorf("failed to unset existing primary: %w", err)
		}
		return s.GuardianPhoneNumberRepo.Update(ctx, phone)
	} else if req.IsPrimary != nil && !*req.IsPrimary && phone.IsPrimary {
		// Unsetting primary - need to promote another number
		phone.IsPrimary = false
	}

	return s.GuardianPhoneNumberRepo.Update(ctx, phone)
}

// DeletePhoneNumber removes a phone number
func (s *RetainedGuardianService) DeletePhoneNumber(ctx context.Context, phoneID int64) error {
	phone, err := s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}
	// Lock the owning guardian profile so this delete (and the primary-promotion
	// below) serializes with the parents-portal contact path's wholesale phone
	// replace (see UpdateGuardian).
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, phone.GuardianProfileID); err != nil {
		return fmt.Errorf(errMsgGuardianNotFound, err)
	}

	wasPrimary := phone.IsPrimary
	guardianID := phone.GuardianProfileID

	// Delete the phone number
	if err := s.GuardianPhoneNumberRepo.Delete(ctx, phoneID); err != nil {
		return fmt.Errorf("failed to delete phone number: %w", err)
	}

	// If deleted number was primary, promote the next one
	if wasPrimary {
		phones, err := s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, guardianID)
		if err != nil {
			return nil // Deletion succeeded, just couldn't promote - not fatal
		}

		// Promote the first remaining phone number (already sorted by priority)
		if len(phones) > 0 {
			_ = s.GuardianPhoneNumberRepo.SetPrimary(ctx, phones[0].ID, guardianID)
		}
	}

	return nil
}

// SetPrimaryPhone sets a phone number as the primary contact
func (s *RetainedGuardianService) SetPrimaryPhone(ctx context.Context, phoneID int64) error {
	phone, err := s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}
	// Lock the owning guardian profile so this primary-flag flip serializes with
	// the parents-portal contact path's wholesale phone replace (delete-all +
	// re-insert) — see UpdateGuardian. Without it a staff set-primary could land
	// between the parent's read-old-phones and its replace, then be wiped by the
	// replace. The phone→profile association is immutable, so reading the phone
	// first and then locking its profile is safe. Held for the request tx.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, phone.GuardianProfileID); err != nil {
		return fmt.Errorf(errMsgGuardianNotFound, err)
	}

	return s.GuardianPhoneNumberRepo.SetPrimary(ctx, phoneID, phone.GuardianProfileID)
}

// GetGuardianPhoneNumbers retrieves all phone numbers for a guardian, sorted by priority
func (s *RetainedGuardianService) GetGuardianPhoneNumbers(ctx context.Context, guardianID int64) ([]*peopleValues.GuardianProfilePhoneSnapshot, error) {
	return s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, guardianID)
}

// GetPhoneNumberByID retrieves a phone number by ID
func (s *RetainedGuardianService) GetPhoneNumberByID(ctx context.Context, phoneID int64) (*peopleValues.GuardianProfilePhoneSnapshot, error) {
	return s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
}
