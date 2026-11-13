package enrollmenthttp_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestRolloverHolidayWishCanBeAnsweredWithBookingAdjustment(t *testing.T) {
	t.Parallel()
	for _, wish := range []bool{true, false} {
		t.Run(fmt.Sprint(wish), func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			env := setupRolloverTest(t)
			ctx := testpkg.Ctx(t)
			child := seedApprovedChild(t, env, env.sourcePhase.ID,
				"Anna", "Wunsch", "anna.wunsch@example.test", "Lina", "Wunsch", 2)
			offering := sourceOffering(t, env, &offeringContract.CareOfferingRecord{
				Name: "Nachmittagsbetreuung", DaysOfWeekMode: offeringContract.DaysOfWeekModeParentChoice,
				AvailableDays: []string{"mon", "tue"}, IsActive: true,
			})
			linkChildOffering(t, env, &enrollment.RequestChildOffering{
				RequestChildID: child.ID, CareOfferingID: offering.ID, SelectedDays: []string{"tue"},
			})
			schema, err := NewTestFormSchemas(env.repos.Enrollment()).CreateSchemaWithLegal(ctx,
				"Ferienwunsch im neuen Schuljahr", nil, env.creatorID,
				enrollment.CoreRequirements{enrollment.CoreRequirementHolidayCareWish: true}, nil)
			require.NoError(t, err)
			input := validRolloverRequest(env, enrollment.PhaseRolloverModeOptOut, true)
			input.FormSchemaID = &schema.ID
			result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, input)
			require.NoError(t, err)
			rolled, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses,
				result.Phase.ID, []string{enrollment.ChildStatusAutoRenewed})
			require.NoError(t, err)
			require.Len(t, rolled, 1)
			request, err := ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), rolled[0].RequestID)
			require.NoError(t, err)
			draft, err := env.requestSvc.GetEditDraft(ctx, request.StatusToken)
			require.NoError(t, err)
			require.Nil(t, draft.Children[0].HolidayCareWish, "the new school year needs a fresh answer")
			links := draft.OfferingsByChild[rolled[0].ID]
			require.Len(t, links, 1)
			svc := newChangeRequestServiceForTest(&requestTestEnv{
				db: env.db, repos: env.repos, reposCare: env.reposCare, reposSchools: env.reposSchools,
				settings: env.settings, outbox: env.outbox, creatorID: env.creatorID,
			})
			confirmation := acceptedRenewalTerms(t, ctx, env.requestSvc, request.StatusToken)
			proposedFlags := map[string]any{"photo": true}
			for key := range confirmation.ConsentFlags.All() {
				proposedFlags[key] = false
			}
			created, err := svc.Create(ctx, request.StatusToken, enrollmentAPI.CreateChangeRequestInput{
				ParentNote: "Bitte Montag statt Dienstag buchen.",
				Submission: enrollmentAPI.SubmitRequest{
					GuardianFirstName: request.GuardianFirstName, GuardianLastName: request.GuardianLastName,
					GuardianEmail: request.GuardianEmail, GuardianPhone: request.GuardianPhone,
					ConsentFlags: testFormValues(proposedFlags), CustomData: request.CustomData,
					Children: []enrollmentAPI.SubmitChild{{
						ID: rolled[0].ID, FirstName: child.FirstName, LastName: child.LastName, DateOfBirth: child.DateOfBirth,
						TargetGradeLevel: rolled[0].TargetGradeLevel, HolidayCareWish: &wish,
						OfferingIDs:  []int64{links[0].CareOfferingID},
						OfferingDays: []enrollmentAPI.SubmitOfferingDays{{OfferingID: links[0].CareOfferingID, SelectedDays: []string{"mon"}}},
					}},
				},
			})
			require.NoError(t, err)
			_, err = env.requestSvc.ConfirmRenewal(ctx, request.StatusToken, confirmation)
			require.NoError(t, err)
			_, err = testpkg.TenantQuery2(t, ctx, env.db, svc.Approve, created.ChangeRequest.ID, enrollmentAPI.ReviewChangeRequestInput{
				ActorAccountID: env.creatorID, ActorRole: "admin",
			})
			require.NoError(t, err)
			stored, saved, err := env.requestSvc.GetByStatusToken(ctx, request.StatusToken)
			require.NoError(t, err)
			require.Len(t, stored.LegalBlocksSnapshot, 1, "approval keeps the renewal evidence")
			assert.Equal(t, true, testFormValue(stored.ConsentFlags, "photo"), "approval keeps the proposed optional consent")
			for key := range confirmation.ConsentFlags.All() {
				assert.Equal(t, true, testFormValue(stored.ConsentFlags, key), "approval keeps accepted %s", key)
			}
			require.NotNil(t, saved[0].HolidayCareWish)
			assert.Equal(t, wish, *saved[0].HolidayCareWish)
			bookings, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().RequestChildOfferingHistory, rolled[0].ID)
			require.NoError(t, err)
			require.Len(t, bookings, 1)
			assert.Equal(t, []string{"mon"}, bookings[0].SelectedDays)
		})
	}
}
