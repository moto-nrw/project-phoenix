package parentportal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The Elternsprechtag (#4801) reaches a child only through its guardian
// relationship: an account without one sees and books nothing, seeing needs
// parent_portal.access and booking parent_portal.conference.book.
func TestParentConferenceChildFollowsTheRelationshipPermissions(t *testing.T) {
	t.Parallel()
	svc, db, _, _ := buildConsentService(t)
	ctx := testpkg.Ctx(t)
	family := testpkg.CreateTestParentGuardianChain(t, db)
	stranger := testpkg.CreateTestParentGuardianChain(t, db)

	for _, book := range []bool{false, true} {
		_, err := svc.ParentConferenceChild(ctx, stranger.AccountID, family.StudentID, book)
		require.ErrorIs(t, err, parentService.ErrChildNotLinked, "a parent without a relationship to the child, book=%v", book)
	}

	child, err := svc.ParentConferenceChild(ctx, family.AccountID, family.StudentID, true)
	require.NoError(t, err, "a full guardian books")
	assert.Equal(t, family.TenantID, child.TenantID)
	assert.Equal(t, family.GuardianProfileID, child.GuardianProfileID)

	_, err = db.ExecContext(context.Background(), `
		UPDATE auth.guardian_student_access AS access
		SET permissions = access.permissions - 'parent_portal.conference.book'
		FROM users.student_guardian_relationships AS relationship
		WHERE relationship.tenant_id = access.tenant_id AND relationship.id = access.relationship_id
			AND relationship.student_id = ? AND relationship.guardian_profile_id = ?`,
		family.StudentID, family.GuardianProfileID)
	require.NoError(t, err)

	_, err = svc.ParentConferenceChild(ctx, family.AccountID, family.StudentID, true)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "booking needs parent_portal.conference.book")
	_, err = svc.ParentConferenceChild(ctx, family.AccountID, family.StudentID, false)
	require.NoError(t, err, "seeing the days needs parent_portal.access only")
}
