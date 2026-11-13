package contracttest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/parentrequests"
	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The parents change one module and keep another (#6334). The office sees
// the kept module as "Bleibt gebucht" next to the change, in the first read
// and in the preview of another date, and compares against the bookings of
// the date it checks.
func TestTraegerChangeShowsTheKeptModuleNextToTheChange(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Bleibt")
	kept := createAdjustmentCareOfferingWith(t, env, "Bleibt Bleibt", func(o *offeringContract.CareOfferingRecord) { o.SortOrder = 203 })
	_, keptChildID, keptStudentID := submitApprovedAdjustmentChild(t, env, "offer-change-kept@example.com", "OfferChangeKept",
		[]*offeringContract.CareOfferingRecord{fx.oldOffering, kept})
	fx.childID, fx.studentID = keptChildID, keptStudentID
	env.settings.mu.Lock()
	env.settings.stringValues[traegerDecisionReasonPolicy] = parentrequests.ReasonPolicyGuardians
	env.settings.mu.Unlock()
	_, err := testpkg.TenantQuery1(t, offeringChangeAdminContext(t), env.db, svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []careplan.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: kept.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
	office := owners.CareContractChangeRequests{Changes: svc, Settings: env.settings}
	later := fx.switchDate.AddDays(7)

	require.NoError(t, inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		first, _, _, err := office.PendingChanges(ctx, fx.studentID, fx.childID, nil)
		require.NoError(t, err)
		require.Len(t, first, 1)
		assertKeptModule(t, first[0].Lines, kept.ID, "first read")
		assertChangeState(t, first[0].Lines, fx.oldOffering.ID, "removed")
		assertChangeState(t, first[0].Lines, fx.newOffering.ID, "added")

		preview, _, _, err := office.PendingChanges(ctx, fx.studentID, fx.childID, &later)
		require.NoError(t, err)
		require.Len(t, preview, 1)
		assertKeptModule(t, preview[0].Lines, kept.ID, "preview of another date")
		return nil
	}))

	// The kept module's booking ends before the later date: there it is
	// booked anew, not kept.
	_, err = env.db.NewRaw(`UPDATE enrollment.care_offering_bookings SET valid_until = ?
  WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?`,
		later.AddDays(-1), testpkg.Tenant(t), fx.childID, kept.ID).Exec(context.Background())
	require.NoError(t, err)
	require.NoError(t, inTraegerDecisionTx(t, testpkg.Ctx(t), env.db, func(ctx context.Context) error {
		preview, _, _, err := office.PendingChanges(ctx, fx.studentID, fx.childID, &later)
		require.NoError(t, err)
		require.Len(t, preview, 1)
		assertChangeState(t, preview[0].Lines, kept.ID, "added")
		return nil
	}))
}

func assertKeptModule(t *testing.T, lines []api.ContractChangeLineForTest, offeringID int64, read string) {
	t.Helper()
	var found []api.ContractChangeLineForTest
	for _, line := range lines {
		if line.CareOfferingID == offeringID {
			found = append(found, line)
		}
	}
	require.Len(t, found, 1, "%s: the kept module appears exactly once", read)
	assert.Equal(t, "unchanged", found[0].State, read)
	assert.Equal(t, "Bleibt Bleibt", found[0].ModuleName, read)
	assert.Equal(t, []string{"mon"}, found[0].BeforeDays, read)
	assert.Equal(t, []string{"mon"}, found[0].AfterDays, read)
}

func assertChangeState(t *testing.T, lines []api.ContractChangeLineForTest, offeringID int64, state string) {
	t.Helper()
	for _, line := range lines {
		if line.CareOfferingID == offeringID {
			assert.Equal(t, state, line.State)
			return
		}
	}
	t.Fatalf("offering %d is missing from the comparison", offeringID)
}
