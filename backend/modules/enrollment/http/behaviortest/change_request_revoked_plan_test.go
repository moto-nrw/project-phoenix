package enrollmenthttp_test

import (
	"testing"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Request-level weekly answers feed the deferred plan just like child-level
// answers. Revocation must refuse their replacement without cancelling the
// plan the school already approved; unrelated guardian answers stay editable.
func TestChangeRequestService_RevokedParentCannotChangeRequestWeeklyPlan(t *testing.T) {
	t.Parallel()
	for _, audience := range []string{enrollment.PhaseAudienceOpen, enrollment.PhaseAudienceLinkedParents} {
		for _, key := range []string{"pickup_times", "arrival_times", "allowed_modes", "remove_pickup", "note"} {
			t.Run(audience+"/"+key, func(t *testing.T) {
				testpkg.OwnTenant(t)
				db := testpkg.SetupTestDB(t)
				r := setupFutureRenewal(t)
				ctx := testpkg.Ctx(t)
				schema, err := NewTestFormSchemas(r.env.repos.Enrollment()).CreateSchema(ctx, "Wochenplan der Familie", []enrollment.FormField{
					{Key: "pickup_times", Label: "Abholzeiten", Type: enrollment.FormFieldWeekdaySchedule, Target: enrollment.TargetSchedulePickup, AllowedTimes: []string{"15:10", "16:00"}},
					{Key: "arrival_times", Label: "Betreuungstage", Type: enrollment.FormFieldWeekdaySchedule, Target: enrollment.TargetScheduleArrival},
					{Key: "allowed_modes", Label: "Heimweg", Type: enrollment.FormFieldWeekdayMultiMode, Target: enrollment.TargetStudentAllowedDepartureModes},
					{Key: "note", Label: "Hinweis", Type: enrollment.FormFieldText},
				}, r.env.creatorID)
				require.NoError(t, err)
				accountID := parentAccountLinkedTo(t, r.env.db, r.studentID, securityruntime.GuardianRolePrimaryGuardian)
				_, err = db.NewUpdate().TableExpr("enrollment.requests").
					Set("schema_id = ?", schema.ID).Set("guardian_account_id = ?", accountID).
					Where("id = ?", r.requestID).Exec(ctx)
				require.NoError(t, err)
				_, err = db.NewUpdate().TableExpr("enrollment.phases").
					Set("form_schema_id = ?", schema.ID).Set("audience = ?", audience).
					Where("id = ?", r.env.sourcePhase.ID).Exec(ctx)
				require.NoError(t, err)
				child, err := ReadOwnerChildForTest(ctx, r.env.repos.Enrollment(), r.childID)
				require.NoError(t, err)
				req, err := ReadOwnerRequestForTest(ctx, r.env.repos.Enrollment(), r.requestID)
				require.NoError(t, err)
				req.CustomData = child.CustomData.Clone()
				req.CustomData.Unset("health")
				req.CustomData.Set("note", "Bisheriger Hinweis")
				child.CustomData = enrollment.FormValues{}
				grade := int16(3)
				child.TargetGradeLevel = &grade
				require.NoError(t, UpdateOwnerChildForTest(ctx, r.env.repos.Enrollment(), child))
				require.NoError(t, UpdateOwnerRequestGuardianForTest(ctx, r.env.repos.Enrollment(), req, false))
				r.approve(t)

				proposed := enrollmentAPI.SubmitRequest{
					PhaseID: req.PhaseID, GuardianFirstName: req.GuardianFirstName, GuardianLastName: req.GuardianLastName,
					GuardianEmail: req.GuardianEmail, GuardianPhone: req.GuardianPhone, ConsentFlags: req.ConsentFlags,
					CustomData: req.CustomData.Clone(), Children: []enrollmentAPI.SubmitChild{{
						ID: child.ID, FirstName: child.FirstName, LastName: child.LastName, DateOfBirth: child.DateOfBirth,
						TargetGradeLevel: child.TargetGradeLevel, CustomData: child.CustomData,
					}},
				}
				switch key {
				case "pickup_times":
					proposed.CustomData.Set(key, map[string]any{"mon": "16:00"})
				case "arrival_times":
					proposed.CustomData.Set(key, map[string]any{"tue": "07:45"})
				case "allowed_modes":
					proposed.CustomData.Set(key, map[string]any{"mon": []any{"alone"}})
				case "remove_pickup":
					proposed.CustomData.Set("pickup_times", map[string]any{})
				case "note":
					proposed.CustomData.Set(key, "Neuer Hinweis")
				}
				svc := newChangeRequestServiceWithDecisionForTest(t, r.env)
				restoreTakeover := liftTakeoverStamp(t, r.env, r.requestID)
				created, err := svc.Create(ctx, req.StatusToken, enrollmentAPI.CreateChangeRequestInput{Submission: proposed})
				require.NoError(t, err)
				restoreTakeover()
				env := &existingChildEnv{decisionTestEnv: r.env}
				env.revokeGuardianLinks(t, r.studentID)
				_, err = testpkg.TenantQuery2(t, ctx, db, svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
					Note: "Änderung geprüft", ActorAccountID: r.reviewerAccountID, ActorRole: "admin",
				})
				if key == "note" {
					require.NoError(t, err, "unrelated guardian answers remain editable")
				} else {
					require.ErrorIs(t, err, enrollment.ErrChangeRequestParentNotPermitted)
					stored, readErr := ReadOwnerRequestForTest(ctx, r.env.repos.Enrollment(), r.requestID)
					require.NoError(t, readErr)
					assert.Equal(t, req.CustomData, stored.CustomData, "the refused approval rolls back the request answers")
				}
				r.assertApprovedPlanAppliesAtStart(t, env, accountID)
			})
		}
	}
}
