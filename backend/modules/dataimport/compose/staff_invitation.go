package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// NewStaffInviter binds the narrow import command to the invitation owner.
// A nil owner preserves the disabled-invitation configuration.
func NewStaffInviter(invitations identityaccess.SchoolInvitations) dataimport.StaffInviter {
	if invitations == nil {
		return nil
	}
	return staffInviter{invitations: invitations}
}

type staffInviter struct {
	invitations identityaccess.SchoolInvitations
}

func (s staffInviter) InviteStaff(ctx context.Context, invitation dataimport.StaffInvitation) error {
	_, err := s.invitations.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: invitation.Email, RoleID: invitation.RoleID, TenantID: invitation.TenantID,
		FirstName: invitation.FirstName, LastName: invitation.LastName, Position: invitation.Position,
		PersonID: invitation.PersonID, CreatedBy: invitation.CreatedBy, SchoolName: invitation.SchoolName,
		ActorPermissions: invitation.ActorPermissions,
	})
	return err
}
