package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The school's Soll follows the end of a contract (#4682) as VersionsInForce
// does: a terminated contract books no day after its last day, a withdrawn
// one no day at all. Without the cut the Soll/Ist comparison and the
// overrun report would count care the contract no longer promises.
func TestBookedCareOfTheSchoolEndsWithTheContract(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Soll Kündigung", Slug: fmt.Sprintf("traeger-soll-kuendigung-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Kündigung")
	august := calendar.NewDate(2099, 8, 3)
	terminated, withdrawn := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	for student, status := range map[int64]string{terminated: "gekuendigt", withdrawn: "widerrufen"} {
		contract := testpkg.CreateTestCareContract(t, db, organization, school, student, "versendet", august,
			testpkg.CareContractService{Weekday: "mon", Module: "OGS", EndsAt: "16:00"})
		_, err := db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'wirksam' WHERE contract_id = ?`, contract)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = 'aktiv', concluded_at = NOW(),
			conclusion_method = 'mit_anmeldung' WHERE id = ?`, contract)
		require.NoError(t, err)
		endsOn := any(nil)
		if status == "gekuendigt" {
			endsOn = "2099-08-31"
		}
		_, err = db.ExecContext(ctx, `UPDATE care_contract.contracts SET status = ?, ends_on = ? WHERE id = ?`, status, endsOn, contract)
		require.NoError(t, err)
	}

	id, err := tenant.NewTenantID(school)
	require.NoError(t, err)
	require.NoError(t, tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		booked, err := compose.NewBookedCare().BookedCare(txCtx, []int64{terminated, withdrawn}, august, calendar.NewDate(2099, 9, 30))
		if err != nil {
			return err
		}
		soll := booked[terminated]
		assert.True(t, soll.Day(calendar.NewDate(2099, 8, 31)).Booked, "the last Monday before the end is booked")
		assert.False(t, soll.Day(calendar.NewDate(2099, 9, 7)).InForce, "after the end the contract books nothing")
		_, found := booked[withdrawn]
		assert.False(t, found, "a withdrawn contract books no day")
		return nil
	}))
}
