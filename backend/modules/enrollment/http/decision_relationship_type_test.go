package enrollmenthttp_test

import (
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An approval links the people of an enrollment form as parents, not as legal
// guardians ("Vormund"), and leaves the type of an existing link alone: only
// the school knows who is a legal guardian (#4930). Parent-portal rights come
// from the guardian role, so the type does not change them.

func TestDecisionService_Decide_NewLinksAreParents(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)

	reqID, childID := submitOneChild(t, env, "first-parent-4930@example.com", "Ida", "Elternteil")
	coGuardianEmail := "co-parent-4930@example.com"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().CreateRequestGuardian, &enrollmentNative.RequestGuardian{
		RequestID: reqID,
		FirstName: "Zweite",
		LastName:  "Elternteil",
		Email:     &coGuardianEmail,
	}))

	outcome, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentNative.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)

	links, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.repos.StudentGuardian.FindByStudentID, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.Len(t, links, 2, "the primary guardian and the co-guardian are linked")
	for _, link := range links {
		assert.Equal(t, "parent", link.RelationshipType, "guardian profile %d", link.GuardianProfileID)
		if link.IsPrimary {
			assertPrimaryGuardianRights(t, link)
		}
	}
}

func TestDecisionService_Decide_ExistingStudentKeepsRelationshipType(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)

	for _, relationshipType := range []string{"parent", "guardian", "relative"} {
		t.Run(relationshipType, func(t *testing.T) {
			existing := testpkg.CreateTestStudent(t, env.db, "Ole", "Bestand-"+relationshipType, "2a")
			guardian := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Maren", "Bestand", "keep-type-"+relationshipType)
			require.NotNil(t, guardian.Email)
			testpkg.CreateTestStudentGuardianLinkOfType(t, env.db, testpkg.Tenant(t), existing.ID, guardian.ID, relationshipType, securityruntime.GuardianRoleLegalGuardian)

			reqID, childID := submitReEnrollment(t, env, "Maren", "Bestand", *guardian.Email, nil,
				"Ole", "Bestand-"+relationshipType, map[string]any{
					"agb": true, "data_processing": true, "email_contact": true, "photo": true,
				})
			matchChildToExistingStudent(t, env, childID, existing.ID)

			_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.decision.Decide, enrollmentAPI.DecideInput{
				RequestID:  reqID,
				ChildID:    childID,
				Status:     enrollmentNative.DecisionApproved,
				ReviewedBy: env.creatorID,
			})
			require.NoError(t, err)

			links, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.repos.StudentGuardian.FindByStudentID, existing.ID)
			require.NoError(t, err)
			require.Len(t, links, 1, "the submitted guardian's existing link is reused")
			assert.Equal(t, guardian.ID, links[0].GuardianProfileID)
			assert.Equal(t, relationshipType, links[0].RelationshipType, "the approval keeps the stored relationship type")
			assert.True(t, links[0].IsPrimary, "the submitted guardian becomes the primary relationship")
			assertPrimaryGuardianRights(t, links[0])
		})
	}
}

// A co-guardian who is already a contact of the child keeps the type the school
// gave them; the form names no type for a co-guardian.
func TestDecisionService_Decide_CoGuardianKeepsContactRelationshipType(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)

	existing := testpkg.CreateTestStudent(t, env.db, "Tom", "Kontakt", "2a")
	contact := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Oma", "Kontakt", "co-guardian-relative")
	require.NotNil(t, contact.Email)
	testpkg.CreateTestStudentGuardianLinkOfType(t, env.db, testpkg.Tenant(t), existing.ID, contact.ID, "relative", securityruntime.GuardianRoleEmergency)

	reqID, childID := submitReEnrollment(t, env, "Vera", "Kontakt", "primary-co-guardian-4930@example.com", nil,
		"Tom", "Kontakt", map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		})
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().CreateRequestGuardian, &enrollmentNative.RequestGuardian{
		RequestID: reqID,
		FirstName: "Oma",
		LastName:  "Kontakt",
		Email:     contact.Email,
	}))
	matchChildToExistingStudent(t, env, childID, existing.ID)

	_, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID:  reqID,
		ChildID:    childID,
		Status:     enrollmentNative.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	links, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), env.repos.StudentGuardian.FindByStudentID, existing.ID)
	require.NoError(t, err)
	var kept, submitted *peopledirectory.GuardianRelationshipSnapshot
	for _, link := range links {
		if link.GuardianProfileID == contact.ID {
			kept = link
		} else if link.IsPrimary {
			submitted = link
		}
	}
	require.NotNil(t, kept, "the contact stays linked")
	assert.Equal(t, "relative", kept.RelationshipType, "the co-guardian keeps the stored relationship type")
	require.NotNil(t, submitted, "the submitted guardian is linked as primary")
	assert.Equal(t, "parent", submitted.RelationshipType, "a new primary link is a parent")
}

// assertPrimaryGuardianRights checks the rights of a primary relationship:
// pickup authority and the parent portal come from the role, whatever the
// relationship type says.
func assertPrimaryGuardianRights(t *testing.T, link *peopledirectory.GuardianRelationshipSnapshot) {
	t.Helper()
	assert.Equal(t, securityruntime.GuardianRolePrimaryGuardian, link.GuardianRole)
	assert.True(t, link.CanPickup)
	assert.True(t, guardianLinkHasPermission(link, securityruntime.GuardianPermissionPortalAccess),
		"a %q link keeps parent-portal access", link.RelationshipType)
}
