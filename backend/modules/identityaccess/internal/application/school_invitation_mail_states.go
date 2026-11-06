package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// SchoolInvitationMailStateReader reads what became of the last mail of each
// invitation. Delivery owns the answer; the root binds it.
type SchoolInvitationMailStateReader func(ctx context.Context, tenantID int64, invitationIDs []int64) (map[int64]identityaccess.InvitationMailState, error)

// SchoolInvitationMailStates reports the mail state of the listed invitations in
// one read, in the caller's tenant transaction (#5934). The list itself stays one
// statement.
func (s *SchoolInvitation) SchoolInvitationMailStates(ctx context.Context, tenantID int64, invitationIDs []int64) (map[int64]identityaccess.InvitationMailState, error) {
	if s == nil {
		return nil, errSchoolInvitationUnavailable
	}
	if s.mailStates == nil {
		return nil, nil
	}
	return s.mailStates(ctx, tenantID, invitationIDs)
}

// refuseDelayedMail stops a resend while the mail server still tries the last
// mail: a second one would only arrive twice.
func (s *SchoolInvitation) refuseDelayedMail(ctx context.Context, tenantID, invitationID int64) error {
	states, err := s.SchoolInvitationMailStates(ctx, tenantID, []int64{invitationID})
	if err != nil {
		return err
	}
	if states[invitationID].Outcome == domain.InvitationMailOutcomeDelayed {
		return domain.ErrInvitationMailDelayed
	}
	return nil
}
