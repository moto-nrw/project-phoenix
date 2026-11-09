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

// The school's Soll never reads a replaced version (#4681): a change on the
// first day of a version replaces it, and the replaced version holds on no
// day. Here the replacing version ends at the end of 2099 and the replaced
// one names no end, so only the filter, not the order of the versions,
// keeps the replaced Monday out of January 2100.
func TestBookedCareOfTheSchoolLeavesAReplacedVersionOut(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Soll Ersetzt", Slug: fmt.Sprintf("traeger-soll-ersetzt-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, "OGS Ersetzt")
	student := testpkg.UniqueTestTenantID(t)
	august := calendar.NewDate(2099, 8, 3)
	contract := testpkg.CreateTestCareContract(t, db, organization, school, student, "versendet", august,
		testpkg.CareContractService{Weekday: "mon", Module: "Früh", EndsAt: "14:00"})

	// Version 1 takes effect, and a change on its first day replaces it by
	// version 2 with Wednesday only, until the end of 2099.
	_, err := db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'wirksam'
		WHERE contract_id = ? AND version_number = 1`, contract)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'ersetzt'
		WHERE contract_id = ? AND version_number = 1`, contract)
	require.NoError(t, err)
	var replacing int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, ends_on, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner, source)
		VALUES (?, ?, 2, 'wirksam', ?, '2099-12-31', 'Kind', 'Vertrag', 'Schule', 'open_ended', 1, 'month_end',
			false, '', false, '', '', '', 'buchungsaenderung') RETURNING id`,
		organization, contract, august).Scan(ctx, &replacing))
	_, err = db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id,
			contract_version_id, weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, 'wed', 1, 'OGS', '16:00')`,
		organization, replacing)
	require.NoError(t, err)

	id, err := tenant.NewTenantID(school)
	require.NoError(t, err)
	require.NoError(t, tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		booked, err := compose.NewBookedCare().BookedCare(txCtx, []int64{student}, august, calendar.NewDate(2100, 1, 31))
		if err != nil {
			return err
		}
		soll := booked[student]
		require.Len(t, soll.Versions, 1, "the replaced version is not part of the Soll")
		assert.Equal(t, 2, soll.Versions[0].Number)
		assert.False(t, soll.Day(calendar.NewDate(2099, 8, 3)).Booked, "the replaced Monday is not booked")
		assert.True(t, soll.Day(calendar.NewDate(2099, 8, 5)).Booked, "the replacing Wednesday is booked")
		assert.False(t, soll.Day(calendar.NewDate(2100, 1, 4)).InForce,
			"after the replacing version ends, the replaced one does not come back")
		return nil
	}))
}
