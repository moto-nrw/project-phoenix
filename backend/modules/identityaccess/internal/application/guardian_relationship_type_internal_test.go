package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An invitation without a relationship type links the invited person as a
// parent, not as a legal guardian ("Vormund"): a school marks a guardian by
// hand (#4930). A type the caller chose and an existing link stay as they are.

func TestInviteToStudent_LinksWithoutTypeAsParent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	result, err := f.lifecycle.inviteToStudent(tenantContext(), inviteRequest("parent-default@example.test"))
	require.NoError(t, err)

	link, linked := f.guardians.linkOf(relativeStudent, result.GuardianProfileID)
	require.True(t, linked)
	assert.Equal(t, "parent", link.RelationshipType)
}

func TestInviteToStudent_KeepsTheChosenRelationshipType(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	req := inviteRequest("relative@example.test")
	req.RelationshipType = "relative"

	result, err := f.lifecycle.inviteToStudent(tenantContext(), req)
	require.NoError(t, err)

	link, linked := f.guardians.linkOf(relativeStudent, result.GuardianProfileID)
	require.True(t, linked)
	assert.Equal(t, "relative", link.RelationshipType)
}

func TestInviteToStudent_KeepsTheTypeOfAnExistingLink(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	profile := f.guardians.addProfile(domain.GuardianProfile{Email: "vormund@example.test"})
	existing := f.guardians.addLink(domain.StudentGuardianLink{
		StudentID: relativeStudent, GuardianProfileID: profile.ID,
		RelationshipType: "guardian", GuardianRole: "legal_guardian",
	})

	_, err := f.lifecycle.inviteToStudent(tenantContext(), inviteRequest("vormund@example.test"))
	require.NoError(t, err)

	assert.Equal(t, "guardian", f.guardians.links[existing.ID].RelationshipType)
}

func TestApproveInvitation_LinksAsParent(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	profile := f.guardians.addProfile(domain.GuardianProfile{Email: "approved-parent@example.test"})
	invitation := pendingApproval(f, profile.ID, true)

	require.NoError(t, f.lifecycle.approveInvitation(tenantContext(), invitation.ID, relativeActor))

	link, linked := f.guardians.linkOf(relativeStudent, profile.ID)
	require.True(t, linked)
	assert.Equal(t, "parent", link.RelationshipType)
}
