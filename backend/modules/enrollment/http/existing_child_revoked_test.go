package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A submit-time pin rests on the parent's relationship to the child. When
// the school removes that relationship before the approval, or before a
// later confirmed edit, the pin no longer authorizes anything: no guardian
// link, no portal access, no form data (#6053).

// revokeGuardianLinks removes every guardian relationship of the student,
// as the school's guardian management does.
func (env *existingChildEnv) revokeGuardianLinks(t *testing.T, studentID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	links, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.StudentGuardian.FindByStudentID, studentID)
	require.NoError(t, err)
	require.NotEmpty(t, links)
	for _, link := range links {
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.StudentGuardian.Delete, any(link.ID)))
	}
	require.Empty(t, env.guardianLinks(t, studentID))
}

// portalAccessRows counts the account's guardian portal access rows of the
// test's school.
func (*existingChildEnv) portalAccessRows(t *testing.T, accountID int64) int {
	t.Helper()
	count, err := testpkg.SetupTestDB(t).NewSelect().TableExpr(`auth.guardian_student_access AS a`).
		Where("a.tenant_id = ?", testpkg.Tenant(t)).
		Where("a.account_id = ?", accountID).
		Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return int(count)
}

func TestExistingChild_RevokedRelationshipBeforeApprovalAsksTheOffice(t *testing.T) {
	t.Parallel()
	for _, audience := range []string{enrollmentNative.PhaseAudienceOpen, enrollmentNative.PhaseAudienceLinkedParents} {
		t.Run(audience, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, audience)
			existing := enrolledChild(t, env.db, "Lina", "Entzogen", existingChildBirthday)
			accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
			res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
				submitChild("Lina", "Entzogen", existingChildBirthday),
			}})
			child := res.Children[0]
			require.NotNil(t, child.MatchedStudentID, "the authorized parent's child is pinned at submission")

			env.revokeGuardianLinks(t, existing.ID)
			require.Zero(t, env.portalAccessRows(t, accountID))

			err := env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID})
			require.ErrorIs(t, err, enrollmentNative.ErrChildPossiblyEnrolled, "a pin without current authority goes to the office")
			assert.Equal(t, enrollmentNative.ChildStatusSubmitted, env.child(t, child.ID).Status)
			assert.Empty(t, env.guardianLinks(t, existing.ID), "the refused approval restores no guardian link")
			assert.Zero(t, env.portalAccessRows(t, accountID), "the refused approval restores no portal access")
			candidates := env.candidates(t, res.Request.ID, child.ID)
			require.Len(t, candidates, 1, "the office sees the enrolled child as a candidate")
			assert.Equal(t, existing.ID, candidates[0].StudentID)

			outcome := env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID})
			assert.Nil(t, outcome.PendingInvite, "the office's choice invites nobody")
			renewed := env.child(t, child.ID)
			require.NotNil(t, renewed.CreatedStudentID)
			assert.Equal(t, existing.ID, *renewed.CreatedStudentID)
			assert.True(t, renewed.MatchedByOffice, "the renewal is enrollment-only")
			assert.Equal(t, 1, studentsNamed(t, env.db, "Lina", "Entzogen"))
			assert.Empty(t, env.guardianLinks(t, existing.ID), "the renewal restores no guardian link")
			assert.Zero(t, env.portalAccessRows(t, accountID), "the renewal restores no portal access")
		})
	}
}

// The office may also decide the revoked family's child is a new child: the
// lapsed pin is dropped and the approval creates one, as for any office
// "create new" choice.
func TestExistingChild_RevokedRelationshipOfficeMayCreateNew(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Mats", "Entzogen", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Mats", "Entzogen", existingChildBirthday),
	}})
	child := res.Children[0]
	require.NotNil(t, child.MatchedStudentID)
	env.revokeGuardianLinks(t, existing.ID)

	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID, CreateNewStudent: true}))
	created := env.child(t, child.ID)
	require.NotNil(t, created.CreatedStudentID)
	assert.NotEqual(t, existing.ID, *created.CreatedStudentID, "the office's choice creates a new child")
	assert.Nil(t, created.MatchedStudentID, "the lapsed pin is dropped")
	assert.Equal(t, 2, studentsNamed(t, env.db, "Mats", "Entzogen"))
	assert.Empty(t, env.guardianLinks(t, existing.ID), "the enrolled child gets no guardian link back")
}

// Authority that still holds keeps the automatic renewal, and a relationship
// revoked after the approval is not restored by a later confirmed edit.
func TestExistingChild_ValidRelationshipRenewsAndLaterRevocationSticks(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Nele", "Gueltig", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
		submitChild("Nele", "Gueltig", existingChildBirthday),
	}})
	child := res.Children[0]
	require.NotNil(t, child.MatchedStudentID)

	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}))
	renewed := env.child(t, child.ID)
	require.NotNil(t, renewed.CreatedStudentID)
	assert.Equal(t, existing.ID, *renewed.CreatedStudentID, "a still-authorized pin renews automatically")
	assert.False(t, renewed.MatchedByOffice)
	assert.NotEmpty(t, env.guardianLinks(t, existing.ID), "the authorized parent stays linked")
	assert.Equal(t, 1, studentsNamed(t, env.db, "Nele", "Gueltig"))

	env.revokeGuardianLinks(t, existing.ID)
	_, err := changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: res.Request.ID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	assert.Empty(t, env.guardianLinks(t, existing.ID), "a later edit restores no revoked guardian link")
	assert.Zero(t, env.portalAccessRows(t, accountID), "a later edit restores no revoked portal access")
}

func TestExistingChild_RevocationSurvivesAudienceChange(t *testing.T) {
	t.Parallel()
	for _, audience := range []string{enrollmentNative.PhaseAudienceExistingStudents, enrollmentNative.PhaseAudienceNewStudents} {
		t.Run(audience, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
			existing := enrolledChild(t, env.db, "Lina", "Zielgruppe", existingChildBirthday)
			accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
			res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{submitChild("Lina", "Zielgruppe", existingChildBirthday)}})
			child := res.Children[0]
			require.NotNil(t, child.MatchedStudentID)
			env.revokeGuardianLinks(t, existing.ID)
			_, err := env.db.NewUpdate().TableExpr(`enrollment.phases`).Set("audience = ?", audience).Where("id = ?", env.sourcePhase.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			require.ErrorIs(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}), enrollmentNative.ErrChildPossiblyEnrolled)
			require.Len(t, env.candidates(t, res.Request.ID, child.ID), 1)
			env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID})
			assert.Empty(t, env.guardianLinks(t, existing.ID))
			assert.Zero(t, env.portalAccessRows(t, accountID))
		})
	}
}

func TestExistingChild_LapsedPinWithoutCandidatesStillExposesOfficeChoice(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
	existing := enrolledChild(t, env.db, "Mats", "Umbenannt", existingChildBirthday)
	accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
	res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{submitChild("Mats", "Umbenannt", existingChildBirthday)}})
	child := res.Children[0]
	require.NotNil(t, child.MatchedStudentID)
	env.revokeGuardianLinks(t, existing.ID)
	_, err := env.db.NewUpdate().TableExpr(`users.persons`).Set("last_name = ?", "Korrigiert").Where("id = ?", existing.PersonID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	require.ErrorIs(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}), enrollmentNative.ErrChildPossiblyEnrolled)
	summary, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.decision.Get, res.Request.ID)
	require.NoError(t, err)
	candidates, requiresChoice := summary.ExistingChildCandidates[child.ID]
	require.True(t, requiresChoice, "the detail exposes a required choice independently of candidate count")
	assert.Empty(t, candidates)
	assert.Zero(t, env.portalAccessRows(t, accountID), "the refused approval restores no portal access")
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID, CreateNewStudent: true}))
	created := env.child(t, child.ID)
	assert.Nil(t, created.MatchedStudentID)
	require.NotNil(t, created.CreatedStudentID)
	assert.NotEqual(t, existing.ID, *created.CreatedStudentID)
	assert.Empty(t, env.guardianLinks(t, existing.ID))
}

func TestExistingChild_ApprovedRevocationSurvivesAudienceChange(t *testing.T) {
	t.Parallel()
	for _, audience := range []string{enrollmentNative.PhaseAudienceExistingStudents, enrollmentNative.PhaseAudienceNewStudents} {
		t.Run(audience, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
			existing := enrolledChild(t, env.db, "Nele", "Zielgruppe", existingChildBirthday)
			accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
			res := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{submitChild("Nele", "Zielgruppe", existingChildBirthday)}})
			child := res.Children[0]
			_, err := env.db.NewUpdate().TableExpr(`enrollment.phases`).Set("audience = ?", audience).Where("id = ?", env.sourcePhase.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}), "a valid relationship renews after an audience change")
			env.revokeGuardianLinks(t, existing.ID)
			_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), enrollmentAPI.SyncApprovedChildDataInput{RequestID: res.Request.ID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true})
			require.NoError(t, err)
			assert.Empty(t, env.guardianLinks(t, existing.ID), "a later edit cannot restore access after an audience change")
			assert.Zero(t, env.portalAccessRows(t, accountID))
		})
	}
}
