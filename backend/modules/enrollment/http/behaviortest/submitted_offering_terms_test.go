package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// #6837: parents submit and sign „Mo bis Fr fest, 80 €“. The school then
// edits the catalog entry to Mo bis Do, 95 €. The submitted choice keeps the
// signed terms, the office's detail names the change, and the approval books
// the new terms only once the office confirms them.
func TestApproval_RefusesOfferingTermsChangedSinceSubmission(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	price := 8000
	offering := createAdjustmentCareOfferingWith(t, env, "Ganztag fest", func(o *offeringContract.CareOfferingRecord) {
		o.DaysOfWeekMode = offeringContract.DaysOfWeekModeFixed
		o.PriceCents = &price
	})

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Snapshot",
		GuardianEmail: "snapshot-6837@example.com",
		ConsentFlags: testFormValues(map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		}),
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Ella", LastName: "Snapshot",
			DateOfBirth:      calendar.NewDate(2019, 5, 6),
			TargetGradeLevel: testpkg.Int16Ptr(1),
			OfferingIDs:      []int64{offering.ID},
		}},
	})
	require.NoError(t, err)
	requestID, childID := submitted.Request.ID, submitted.Children[0].ID

	_, err = env.db.NewRaw(`UPDATE enrollment.care_offerings
		SET price_cents = 9500, available_days = '["mon","tue","wed","thu"]'::jsonb
		WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), offering.ID).Exec(ctx)
	require.NoError(t, err)

	sets, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.ListChildOfferings, requestID)
	require.NoError(t, err)
	rows := sets[childID].Current
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].SubmittedTerms, "the choice keeps the terms the parents signed")
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, rows[0].SubmittedTerms.AvailableDays)
	require.NotNil(t, rows[0].SubmittedTerms.PriceCents)
	assert.Equal(t, 8000, *rows[0].SubmittedTerms.PriceCents)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, rows[0].AvailableDays, "the row still shows the current catalog")
	assert.Equal(t, []enrollmentNative.OfferingTermChange{enrollmentNative.OfferingTermPrice, enrollmentNative.OfferingTermDays}, rows[0].ChangedTerms)

	approve := enrollmentAPI.DecideInput{
		RequestID: requestID, ChildID: childID,
		Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
	}
	_, err = testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, approve)
	require.ErrorIs(t, err, enrollmentNative.ErrOfferingTermsChanged)
	child, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().ChildByID, childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusSubmitted, child.Status, "a refused approval writes nothing")

	approve.AcceptOfferingChanges = true
	outcome, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, approve)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusApproved, outcome.Child.Status)
}
