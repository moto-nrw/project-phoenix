package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Every approval and every later edit asks again whether the request's
// identity still holds its relationship to the child, also without a portal
// account and for a child the approval created (#6173).

// accountlessGuardianOf links a guardian profile without an account to the
// student and returns its email.
func accountlessGuardianOf(t *testing.T, env *existingChildEnv, studentID int64) string {
	t.Helper()
	profile := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Eva", "Ohnekonto", "accountless-parent")
	testpkg.CreateTestStudentGuardianLink(t, env.db, studentID, profile.ID, securityruntime.GuardianRolePrimaryGuardian)
	return *profile.Email
}

// submitThroughLateInvite submits the child through a late invite the school
// issued to email, with another contact address on the form.
func (env *existingChildEnv) submitThroughLateInvite(t *testing.T, email string, child enrollmentAPI.SubmitChild) *enrollmentAPI.SubmitResult {
	t.Helper()
	ctx := testpkg.Ctx(t)
	invite, err := env.requestSvc.CreateLateInvite(ctx, enrollmentAPI.CreateLateInviteInput{
		PhaseID: env.sourcePhase.ID, GuardianEmail: email, CreatedBy: env.creatorID,
	})
	require.NoError(t, err)
	res, err := env.matching.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID, LateInviteToken: invite.Token,
		GuardianFirstName: "Eva", GuardianLastName: "Ohnekonto", GuardianEmail: "contact-" + email,
		ConsentFlags: testFormValues(rolloverConsentFlags()), Children: []enrollmentAPI.SubmitChild{child},
	})
	require.NoError(t, err)
	require.Len(t, res.Children, 1)
	require.Equal(t, enrollmentNative.RequestSourceLateInvite, res.Request.SubmissionSource)
	return res
}

func TestExistingChild_AccountlessPinInExistingStudentsPhaseRechecksAtApproval(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceExistingStudents)
	existing := enrolledChild(t, env.db, "Ida", "Ohnekonto", existingChildBirthday)
	email := accountlessGuardianOf(t, env, existing.ID)
	res := env.submitThroughLateInvite(t, email, submitChild("Ida", "Ohnekonto", existingChildBirthday))
	child := res.Children[0]
	require.NotNil(t, child.MatchedStudentID, "the invited guardian's relationship authorizes the renewal at submission")
	env.revokeGuardianLinks(t, existing.ID)

	err := env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID})
	require.ErrorIs(t, err, enrollmentNative.ErrChildPossiblyEnrolled, "a lapsed accountless pin goes to the office")
	assert.Empty(t, env.guardianLinks(t, existing.ID), "the refused approval restores no guardian link")
	require.Len(t, env.candidates(t, res.Request.ID, child.ID), 1)
	require.ErrorIs(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID, CreateNewStudent: true}),
		enrollmentNative.ErrExistingChildChoiceInvalid, "an existing_students phase never creates a second child")

	outcome := env.decideOutcome(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID, ExistingStudentID: &existing.ID})
	assert.Nil(t, outcome.PendingInvite, "the enrollment-only renewal invites nobody")
	renewed := env.child(t, child.ID)
	require.NotNil(t, renewed.CreatedStudentID)
	assert.Equal(t, existing.ID, *renewed.CreatedStudentID)
	assert.True(t, renewed.MatchedByOffice, "the renewal is enrollment-only")
	assert.Equal(t, 1, studentsNamed(t, env.db, "Ida", "Ohnekonto"))
	assert.Empty(t, env.guardianLinks(t, existing.ID), "the renewal restores no guardian link")
}

func TestExistingChild_AccountlessPinStillAuthorizedRenewsWithFamily(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceExistingStudents)
	existing := enrolledChild(t, env.db, "Ole", "Ohnekonto", existingChildBirthday)
	email := accountlessGuardianOf(t, env, existing.ID)
	res := env.submitThroughLateInvite(t, email, submitChild("Ole", "Ohnekonto", existingChildBirthday))
	child := res.Children[0]
	require.NotNil(t, child.MatchedStudentID)

	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}))
	renewed := env.child(t, child.ID)
	assert.False(t, renewed.MatchedByOffice, "a still-authorized pin renews automatically")
	assert.NotEmpty(t, env.guardianLinks(t, existing.ID))

	env.revokeGuardianLinks(t, existing.ID)
	_, err := changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), enrollmentAPI.SyncApprovedChildDataInput{
		RequestID: res.Request.ID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true,
	})
	require.NoError(t, err)
	assert.Empty(t, env.guardianLinks(t, existing.ID), "a later edit restores no revoked guardian link")
}

func TestExistingChild_CreatedChildEditRechecksTheRelationship(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"account", "accountless"} {
		t.Run(scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceOpen)
			in := existingChildSubmission{children: []enrollmentAPI.SubmitChild{submitChild("Neo", "Neu"+scenario, existingChildBirthday)}}
			var accountID int64
			if scenario == "account" {
				account := testpkg.CreateTestAccount(t, env.db, "created-child-parent")
				accountID = account.ID
				in.accountID = &accountID
			}
			res := env.submit(t, in)
			child := res.Children[0]
			require.Nil(t, child.MatchedStudentID)
			require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: child.ID}))
			created := env.child(t, child.ID)
			require.NotNil(t, created.CreatedStudentID)
			studentID := *created.CreatedStudentID
			require.NotEmpty(t, env.guardianLinks(t, studentID), "the approval links the family to the new child")

			sync := enrollmentAPI.SyncApprovedChildDataInput{RequestID: res.Request.ID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true}
			_, err := changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), sync)
			require.NoError(t, err)
			require.NotEmpty(t, env.guardianLinks(t, studentID), "an authorized family keeps its link through an edit")

			env.revokeGuardianLinks(t, studentID)
			_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), sync)
			require.NoError(t, err)
			assert.Empty(t, env.guardianLinks(t, studentID), "a later edit restores no removed relationship")
			if scenario == "account" {
				assert.Zero(t, env.portalAccessRows(t, accountID), "a later edit restores no portal access")
			}
			sync.ChildDataChanged = true
			_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(testpkg.Ctx(t), sync)
			require.ErrorIs(t, err, enrollmentNative.ErrChangeRequestParentNotPermitted, "the family may no longer change the child's data")
		})
	}
}

// A rollover request copies the contact address only. The late invite's
// address the approval linked still authorizes later edits of a descendant.
func TestExistingChild_LateInviteRolloverDescendantKeepsInviteeAuthority(t *testing.T) {
	t.Parallel()
	env := setupExistingChildTest(t, enrollmentNative.PhaseAudienceExistingStudents)
	ctx := testpkg.Ctx(t)
	existing := enrolledChild(t, env.db, "Pia", "Ohnekonto", existingChildBirthday)
	email := accountlessGuardianOf(t, env, existing.ID)
	res := env.submitThroughLateInvite(t, email, submitChild("Pia", "Ohnekonto", existingChildBirthday))
	require.NotNil(t, res.Children[0].MatchedStudentID)
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: res.Request.ID, ChildID: res.Children[0].ID}))

	roll := validRolloverRequest(env.rolloverTestEnv, enrollmentNative.PhaseRolloverModeOptOut, true)
	result, err := testpkg.TenantQuery1(t, ctx, env.db, env.rolloverSvc.CreatePhaseFromSource, roll)
	require.NoError(t, err)
	children, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, result.Phase.ID, []string{enrollmentNative.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, children, 1)
	child := children[0]
	require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: child.RequestID, ChildID: child.ID}))
	descendant, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), child.RequestID)
	require.NoError(t, err)
	require.NotEqual(t, email, descendant.GuardianEmail, "the rollover request carries the contact address")

	sync := enrollmentAPI.SyncApprovedChildDataInput{RequestID: child.RequestID, ChildID: child.ID, ActorAccountID: env.creatorID, ReplaceTargetedData: true, ChildDataChanged: true}
	_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(ctx, sync)
	require.NoError(t, err, "the invitee's intact relationship still authorizes the family's edit")
	assert.NotEmpty(t, env.guardianLinks(t, existing.ID))

	env.revokeGuardianLinks(t, existing.ID)
	_, err = changeRequestApplierForTest(t, env.decisionTestEnv).SyncApprovedChildData(ctx, sync)
	require.ErrorIs(t, err, enrollmentNative.ErrChangeRequestParentNotPermitted)
	assert.Empty(t, env.guardianLinks(t, existing.ID))
}
