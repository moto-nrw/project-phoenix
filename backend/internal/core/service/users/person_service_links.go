package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	authService "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
)

// LinkToAccount associates a person with an account
func (s *personService) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	// Verify the account exists
	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if account == nil {
		return &UsersError{Op: opLinkToAccount, Err: authService.ErrAccountNotFound}
	}

	// Check if the account is already linked to another person
	existingPerson, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return &UsersError{Op: opLinkToAccount, Err: ErrAccountAlreadyLinked}
	}

	if err := s.personRepo.LinkToAccount(ctx, personID, accountID); err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	return nil
}

// UnlinkFromAccount removes account association from a person
func (s *personService) UnlinkFromAccount(ctx context.Context, personID int64) error {
	if err := s.personRepo.UnlinkFromAccount(ctx, personID); err != nil {
		return &UsersError{Op: "unlink from account", Err: err}
	}
	return nil
}

// LinkToRFIDCard associates a person with an RFID card
func (s *personService) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	// Check if the RFID card exists, create it if it doesn't (auto-create on assignment)
	card, err := s.rfidRepo.FindByID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if card == nil {
		// Auto-create RFID card on assignment (per RFID Implementation Guide)
		newCard := &userModels.RFIDCard{
			StringIDModel: base.StringIDModel{ID: tagID},
			Active:        true,
		}
		if err := s.rfidRepo.Create(ctx, newCard); err != nil {
			return &UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	// Check if the card is already linked to another person
	existingPerson, err := s.personRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		// Auto-unlink from previous person (tag override behavior)
		if err := s.personRepo.UnlinkFromRFIDCard(ctx, existingPerson.ID); err != nil {
			return &UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	if err := s.personRepo.LinkToRFIDCard(ctx, personID, tagID); err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	return nil
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (s *personService) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	if err := s.personRepo.UnlinkFromRFIDCard(ctx, personID); err != nil {
		return &UsersError{Op: "unlink from RFID card", Err: err}
	}
	return nil
}
