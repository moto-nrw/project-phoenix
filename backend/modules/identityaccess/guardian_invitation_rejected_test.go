package identityaccess_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

func TestGuardianInvitationIsRejected(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]bool{
		identityaccess.GuardianInvitationApprovalRejected:    true,
		identityaccess.GuardianInvitationApprovalApproved:    false,
		identityaccess.GuardianInvitationApprovalNotRequired: false,
	} {
		if got := (identityaccess.GuardianInvitation{ApprovalStatus: status}).IsRejected(); got != want {
			t.Errorf("IsRejected(%q) = %v, want %v", status, got, want)
		}
	}
}
