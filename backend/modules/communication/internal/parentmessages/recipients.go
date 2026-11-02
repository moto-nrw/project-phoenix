package messaging

import (
	"context"
	"fmt"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// authorizeThreadParticipants enforces the shared precondition for opening or
// starting a conversation: the staffer may read the child, messaging is enabled
// for the tenant, and the chosen recipient is an account-holding guardian of the
// child. StartThread and OpenThread both run it so their access rules cannot
// drift apart. Must run inside the tenant transaction.
func (s *Service) authorizeThreadParticipants(ctx context.Context, studentID, guardianAccountID int64) error {
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return err
	}
	if err := s.requireEnabled(ctx); err != nil {
		return err
	}
	// The recipient must be an account-holding guardian of this child.
	guardians, err := s.ThreadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("messaging: list guardians: %w", err)
	}
	if !containsGuardian(guardians, guardianAccountID) {
		return ErrInvalidGuardian
	}
	return nil
}

// ListGuardians lists the recipients for a new conversation about the child
// and who would read it in the audience on the school side (#5200). Only a
// leadership member may ask for the leadership audience.
func (s *Service) ListGuardians(ctx context.Context, studentID int64, audience string) ([]*MessageableGuardian, deliveryPorts.ParentMessageReaderPreview, error) {
	normalized, err := s.staffAudience(ctx, audience)
	if err != nil {
		return nil, deliveryPorts.ParentMessageReaderPreview{}, err
	}
	if err := s.canReadStudent(ctx, studentID); err != nil {
		return nil, deliveryPorts.ParentMessageReaderPreview{}, err
	}
	guardians, err := s.ThreadRepo.ListGuardiansForStudent(ctx, studentID)
	if err != nil {
		return nil, deliveryPorts.ParentMessageReaderPreview{}, fmt.Errorf("messaging: list guardians: %w", err)
	}
	preview, err := s.readerPreview(ctx, normalized)
	if err != nil {
		return nil, deliveryPorts.ParentMessageReaderPreview{}, err
	}
	return guardians, preview, nil
}

// ListReachableStudentIDs returns the school's children that ListGuardians
// would offer at least one recipient for, so the child list of „Neue
// Nachricht“ can mark the others before a child is picked (#4362). It answers
// for every child at once, so only a caller who reads every child may ask.
func (s *Service) ListReachableStudentIDs(ctx context.Context) ([]int64, error) {
	if !s.scope(ctx) {
		return nil, ErrForbidden
	}
	ids, err := s.ThreadRepo.ListStudentIDsWithMessageableGuardian(ctx)
	if err != nil {
		return nil, fmt.Errorf("messaging: list reachable students: %w", err)
	}
	return ids, nil
}

func containsGuardian(guardians []*MessageableGuardian, accountID int64) bool {
	for _, g := range guardians {
		if g != nil && g.AccountID == accountID {
			return true
		}
	}
	return false
}
