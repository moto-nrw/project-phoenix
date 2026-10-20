package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// prepareInvitationReset captures the old number before reinstatement can
// clear a conflicting one, and acquires Workforce locks before Membership.
func (s *SchoolInvitation) prepareInvitationReset(ctx context.Context, invitation domain.SchoolInvitation, accountID int64) (func(context.Context, *domain.SchoolIdentity) error, error) {
	unchanged := func(context.Context, *domain.SchoolIdentity) error { return nil }
	if invitation.EmploymentAction != domain.EmploymentActionResume || (!invitation.ResetMasterData && !invitation.ResetQualifications && !invitation.ResetWorkTime) {
		return unchanged, nil
	}
	former, found, err := s.identity.FindFormerEmployment(ctx, accountID)
	if err != nil {
		return nil, failed("read former employment", err)
	}
	if !found {
		return unchanged, nil
	}
	commands := s.employmentReset
	if commands == nil || commands.Prepare == nil || commands.Execute == nil {
		return nil, failed("reset employment", fmt.Errorf("employment reset is not composed"))
	}
	input := identityPorts.EmploymentReset{StaffID: former.StaffID, AccountID: accountID, InvitationID: invitation.ID, MasterData: invitation.ResetMasterData, Qualifications: invitation.ResetQualifications, WorkTime: invitation.ResetWorkTime}
	input.PreviousPersonnelNumber, err = commands.Prepare(ctx, input)
	if err != nil {
		return nil, failed("prepare employment reset", err)
	}
	return func(ctx context.Context, identity *domain.SchoolIdentity) error {
		if identity == nil || identity.StaffID != former.StaffID {
			return nil
		}
		if err := commands.Execute(ctx, input); err != nil {
			return failed("reset employment", err)
		}
		return nil
	}, nil
}
