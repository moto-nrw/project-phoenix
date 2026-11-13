package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Rollover descendants inherit the student and submitting account, not the
// original match pin. A later family edit still needs current authority.
func TestChangeRequestService_RolloverDescendantRechecksParentAuthority(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"authorized_child_edit", "revoked_child_edit", "revoked_request_note"} {
		t.Run(scenario, func(t *testing.T) {
			testpkg.OwnTenant(t)
			env := setupExistingChildTest(t, enrollment.PhaseAudienceOpen)
			ctx := testpkg.Ctx(t)
			publishLegacyDuplicateTargetSchema(t, env.decisionTestEnv, "rollover-parent-note", []enrollment.FormField{{
				Key: "note", Label: "Hinweis", Type: enrollment.FormFieldText,
			}})
			existing := enrolledChild(t, env.db, "Lina", "Bestand", existingChildBirthday)
			accountID := parentAccountLinkedTo(t, env.db, existing.ID, securityruntime.GuardianRolePrimaryGuardian)
			submitted := env.submit(t, existingChildSubmission{accountID: &accountID, children: []enrollmentAPI.SubmitChild{
				submitChild("Lina", "Bestand", existingChildBirthday),
			}})
			require.NotNil(t, submitted.Children[0].MatchedStudentID)
			require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID}))

			roll := validRolloverRequest(env.rolloverTestEnv, enrollment.PhaseRolloverModeOptOut, true)
			var child *enrollment.RequestChild
			for year := range 2 {
				roll.Name += "-next"
				roll.ServiceStartDate = calendar.NewDate(2027+year, 9, 1)
				roll.ServiceEndDate = calendar.NewDate(2028+year, 7, 31)
				result, err := testpkg.TenantQuery1(t, ctx, env.db, env.rolloverSvc.CreatePhaseFromSource, roll)
				require.NoError(t, err)
				children, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, result.Phase.ID, []string{enrollment.ChildStatusAutoRenewed})
				require.NoError(t, err)
				require.Len(t, children, 1)
				child = env.child(t, children[0].ID)
				require.Nil(t, child.MatchedStudentID)
				require.NotNil(t, child.RolloverSourceChildID)
				require.NoError(t, env.decide(t, enrollmentAPI.DecideInput{RequestID: child.RequestID, ChildID: child.ID}))
				roll.SourcePhaseID = result.Phase.ID
			}
			child = env.child(t, child.ID)
			require.Equal(t, existing.ID, *child.CreatedStudentID)
			plannedClass, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().ChildClassSwitch, child.ID)
			require.NoError(t, err)
			require.NotNil(t, plannedClass)
			req, err := ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), child.RequestID)
			require.NoError(t, err)
			require.Equal(t, accountID, *req.GuardianAccountID)
			proposed := enrollmentAPI.SubmitRequest{
				PhaseID: req.PhaseID, GuardianFirstName: req.GuardianFirstName, GuardianLastName: req.GuardianLastName,
				GuardianEmail: req.GuardianEmail, GuardianPhone: req.GuardianPhone, ConsentFlags: req.ConsentFlags,
				CustomData: req.CustomData, Children: []enrollmentAPI.SubmitChild{{
					ID: child.ID, FirstName: child.FirstName, LastName: child.LastName, DateOfBirth: child.DateOfBirth,
					TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass, CustomData: child.CustomData,
				}},
			}
			if scenario == "revoked_request_note" {
				proposed.CustomData = testFormValues(map[string]any{"note": "Neuer Hinweis"})
			} else {
				proposed.Children[0].LastName = "Geändert"
			}
			svc := newChangeRequestServiceWithDecisionForTest(t, env.decisionTestEnv)
			restoreTakeover := liftTakeoverStamp(t, env.decisionTestEnv, req.ID)
			created, err := svc.Create(ctx, req.StatusToken, enrollmentAPI.CreateChangeRequestInput{Submission: proposed})
			require.NoError(t, err)
			restoreTakeover()
			if scenario != "authorized_child_edit" {
				env.revokeGuardianLinks(t, existing.ID)
			}
			_, err = testpkg.TenantQuery2(t, ctx, env.db, svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
				Note: "Änderung geprüft", ActorAccountID: env.creatorID, ActorRole: "admin",
			})
			if scenario == "revoked_child_edit" {
				require.ErrorIs(t, err, enrollment.ErrChangeRequestParentNotPermitted)
				assert.Equal(t, "Bestand", env.child(t, child.ID).LastName, "the refused approval rolls back the family data")
			} else {
				require.NoError(t, err)
			}
			if scenario != "authorized_child_edit" {
				assert.Empty(t, env.guardianLinks(t, existing.ID), "a descendant edit must not restore removed relationships")
				assert.Zero(t, env.portalAccessRows(t, accountID))
			}
			person, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Person.FindByID, any(existing.PersonID))
			require.NoError(t, err)
			if scenario == "authorized_child_edit" {
				assert.Equal(t, "Geändert", person.LastName)
			} else {
				assert.Equal(t, "Bestand", person.LastName)
				if scenario == "revoked_child_edit" {
					_, err = testpkg.TenantQuery1(t, ctx, env.db, svc.CorrectApprovedChildData, enrollmentAPI.CorrectApprovedChildDataInput{
						RequestID: req.ID, ChildID: child.ID, FirstName: child.FirstName, LastName: "Berichtigt",
						DateOfBirth: child.DateOfBirth, TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass,
						Reason: "Nachname berichtigt", ActorAccountID: env.creatorID,
					})
					require.NoError(t, err)
					person, err = testpkg.TenantQuery1(t, ctx, env.db, env.repos.Person.FindByID, any(existing.PersonID))
					require.NoError(t, err)
					assert.Equal(t, "Berichtigt", person.LastName, "staff corrections still reach the inherited student")
					assert.Empty(t, env.guardianLinks(t, existing.ID))
					assert.Zero(t, env.portalAccessRows(t, accountID))
				}
				afterClass, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().ChildClassSwitch, child.ID)
				require.NoError(t, err)
				assert.Equal(t, plannedClass, afterClass, "the school's approved class switch survives revocation and corrections")
			}
		})
	}
}
