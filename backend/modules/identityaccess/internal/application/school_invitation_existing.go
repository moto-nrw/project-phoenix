package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// existingAccountCredential decides how an invitation to an address that
// already has an account is accepted: a caterer account is refused (ADR
// 0074), a dormant account gets the invitee's new password (restore), and
// any other account must prove its own session.
func (s *SchoolInvitation) existingAccountCredential(ctx context.Context, account domain.LoginAccount, registration domain.InvitationRegistration) (string, bool, error) {
	if err := refuseCatererAccount(ctx, s.logins, account.ID, opAcceptInvitation); err != nil {
		return "", false, err
	}
	dormant, err := s.accountDormant(ctx, account)
	if err != nil {
		return "", false, failed(opAcceptInvitation, err)
	}
	if !dormant {
		if ownerErr := s.verifyOwner(account, registration.OwnerAccessToken); ownerErr != nil {
			return "", false, failed(opAcceptInvitation, ownerErr)
		}
		return "", false, nil
	}
	hash, err := s.hashPassword(registration)
	return hash, err == nil, err
}
