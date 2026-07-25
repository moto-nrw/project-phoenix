package parent

import (
	"context"
	"strings"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

const guardianEmailChangedReason = "guardian email changed"

func normalizeParentGuardianEmail(email *string) string {
	if email == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*email))
}

// revokeInvitationsAfterEmailChange invalidates tokens sent to the previous
// address and stops any matching queued messages before the surrounding tenant
// transaction commits.
func (s *service) revokeInvitationsAfterEmailChange(ctx context.Context, guardianProfileID int64, oldEmail, newEmail string) error {
	if oldEmail == newEmail || s.GuardianInviteRepo == nil {
		return nil
	}
	invitationIDs, err := s.GuardianInviteRepo.RevokeOpenByGuardianProfileID(ctx, guardianProfileID, guardianEmailChangedReason)
	if err != nil {
		return err
	}
	if s.OutboxCanceller == nil {
		return nil
	}
	for _, invitationID := range invitationIDs {
		if _, err := s.OutboxCanceller.CancelPendingByRelatedEntity(
			ctx,
			platformModels.EmailRelatedTypeGuardianInvitation,
			invitationID,
			guardianEmailChangedReason,
		); err != nil {
			return err
		}
	}
	return nil
}
