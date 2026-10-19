package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGuardianInvitationLocksProfileBeforeWriting(t *testing.T) {
	t.Parallel()

	t.Run("direct invitation", func(t *testing.T) {
		fixture := newLifecycleFixture(t)
		profile := fixture.guardians.addProfile(domain.GuardianProfile{TenantID: lifecycleTenant, Email: "parent@example.invalid"})

		_, err := fixture.lifecycle.CreateGuardianInvitation(tenantContext(), domain.GuardianInvitationRequest{
			GuardianProfileID: profile.ID,
			CreatedBy:         1,
		})
		require.NoError(t, err)
		require.Equal(t, []int64{profile.ID}, fixture.guardians.lockedProfiles)
	})

	t.Run("student invitation", func(t *testing.T) {
		fixture := newLifecycleFixture(t)
		profile := fixture.guardians.addProfile(domain.GuardianProfile{TenantID: lifecycleTenant, Email: "parent@example.invalid"})

		_, err := fixture.lifecycle.insertStudentInvitation(tenantContext(), domain.InviteToStudentRequest{
			StudentID: 17,
			CreatedBy: 1,
		}, profile, lifecycleTenant, domain.GuardianInvitationApprovalNotRequired, false, false)
		require.NoError(t, err)
		require.Equal(t, []int64{profile.ID}, fixture.guardians.lockedProfiles)
	})
}
