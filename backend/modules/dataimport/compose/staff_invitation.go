package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// NewStaffInviter binds the narrow import command to the existing invitation
// owner. A nil owner preserves the disabled-invitation configuration.
func NewStaffInviter(service authService.InvitationService) dataimport.StaffInviter {
	if service == nil {
		return nil
	}
	return staffInviter{service: service}
}

type staffInviter struct{ service authService.InvitationService }

func (s staffInviter) InviteStaff(ctx context.Context, invitation dataimport.StaffInvitation) error {
	_, err := s.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: invitation.Email, RoleID: invitation.RoleID, TenantID: invitation.TenantID,
		FirstName: invitation.FirstName, LastName: invitation.LastName, Position: invitation.Position,
		PersonID: invitation.PersonID, CreatedBy: invitation.CreatedBy, SchoolName: invitation.SchoolName,
		ActorPermissions: invitation.ActorPermissions,
	})
	return err
}
