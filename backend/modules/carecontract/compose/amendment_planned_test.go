package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// februarySchool books the module until 14:00 on Monday from August 2099
// and, from an earlier approval, on Monday and Wednesday from 1 March 2100;
// the school approved a change to the module until 16:30 on Monday from
// 1 February 2100.
type februarySchool struct{ changedSchool }

func (s februarySchool) Bookings(context.Context, []int64) (map[int64][]carecontract.Booking, error) {
	from, february, march := calendar.NewDate(2099, 8, 1), calendar.NewDate(2100, 2, 1), calendar.NewDate(2100, 3, 1)
	return map[int64][]carecontract.Booking{s.child: {
		{CareOfferingID: s.offering, Days: []string{"mon"}, ValidFrom: &from, ValidUntil: &february},
		// The earlier approval behind the planned version 2.
		{CareOfferingID: s.offering, Days: []string{"mon", "wed"}, ValidFrom: &march},
		{CareOfferingID: s.offering + 1, Days: []string{"mon"}, ValidFrom: &february},
	}}, nil
}

// plannedContract stores an active contract with version 1 from August 2099
// and version 2 already planned from 1 March 2100 (Monday and Wednesday
// until 14:00).
func plannedContract(t *testing.T, ctx context.Context, organization int64, school februarySchool) int64 {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	var contract int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
			status, sent_at, concluded_at, conclusion_method) VALUES (?, ?, ?, 'aktiv', NOW(), NOW(), 'mit_anmeldung') RETURNING id`,
		organization, school.school.ID, school.child).Scan(ctx, &contract))
	insert := func(number int, status, from, source string, days ...string) {
		var version int64
		require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
				status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
				special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner, source)
			VALUES (?, ?, ?, ?, ?, 'Lina', 'Muster', 'OGS Am Lindenhof', 'open_ended', 1, 'month_end',
				false, '', false, '', '', '', ?) RETURNING id`, organization, contract, number, status, from, source).Scan(ctx, &version))
		for _, day := range days {
			_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
					weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, ?, ?, 'Kurzbetreuung', '14:00')`,
				organization, version, day, school.offering)
			require.NoError(t, err)
		}
	}
	insert(1, "wirksam", "2099-08-01", "abschluss", "mon")
	_, err := db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'abgeloest'
		WHERE contract_id = ? AND version_number = 1`, contract)
	require.NoError(t, err)
	insert(2, "wirksam", "2100-03-01", "buchungsaenderung", "mon", "wed")
	return contract
}

// Invariants 1 and 2 (#4681), the reproduced case: version 2 is planned
// from 1 March, and the school approves a change from 1 February. The change
// becomes version 3 from 1 February to 29 February; version 2 stays as it
// is from 1 March. Every day from January to mid-March has exactly one
// version, and the Mondays of February book the module until 16:30. Before
// the fix the change was dropped, and February billed the old Monday.
func TestPublicChangeBeforeAPlannedVersionHoldsUntilItAndEveryDayHasOneVersion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Änderung Geplant", Slug: fmt.Sprintf("traeger-geplant-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	school := februarySchool{changedSchool{school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS Am Lindenhof"},
		child: testpkg.UniqueTestTenantID(t), offering: testpkg.UniqueTestTenantID(t)}}
	contract := plannedContract(t, ctx, organization, school)
	conclusion := compose.NewConclusion(
		compose.ContractPorts{Letterheads: letterheadDouble{}, Post: &postDouble{}, Runtime: runtimeDouble{}, Access: school},
		compose.IntakePorts{Schools: school, Applications: school, Bookings: school, Access: school, Runtime: runtimeDouble{}, Modes: modesDouble{}})

	run, err := conclusion.AmendFromBookings(ctx, organization)
	require.NoError(t, err)
	assert.Equal(t, []carecontract.AmendedContract{{ContractID: contract, VersionNumber: 3, EffectiveFrom: calendar.NewDate(2100, 2, 1)}},
		run.Amended, "the February change goes in front of the planned March version")
	again, err := conclusion.AmendFromBookings(ctx, organization)
	require.NoError(t, err)
	assert.Empty(t, again.Amended, "a repeated run changes nothing, and the planned version stays")

	versions := compose.NewContractVersions()
	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		from, to := calendar.NewDate(2100, 1, 15), calendar.NewDate(2100, 3, 15)
		periods, err := versions.VersionsInForce(txCtx, organization, from, to, contract)
		if err != nil {
			return err
		}
		numbers := []int{}
		for _, period := range periods {
			numbers = append(numbers, period.Version.Number)
		}
		assert.Equal(t, []int{1, 3, 2}, numbers, "August, the February change, the planned March version")
		for day := from; !day.After(to); day = day.AddDays(1) {
			holding := 0
			for _, period := range periods {
				if !day.Before(period.From) && !day.After(period.Until) {
					holding++
				}
			}
			assert.Equal(t, 1, holding, "exactly one version on %s", day)
		}
		require.Len(t, periods, 3)
		february := periods[1]
		assert.Equal(t, []calendar.Date{calendar.NewDate(2100, 2, 1), calendar.NewDate(2100, 2, 28)},
			[]calendar.Date{february.From, february.Until})
		monday := calendar.NewDate(2100, 2, 8)
		require.Len(t, february.Version.ServicesOn(monday), 1)
		assert.Equal(t, school.offering+1, february.Version.ServicesOn(monday)[0].CareOfferingID,
			"a Monday in February books the module until 16:30")
		march := periods[2]
		assert.Equal(t, 2, march.Version.Number, "the planned version stays as it is")
		assert.Len(t, march.Version.ServicesOn(calendar.NewDate(2100, 3, 3)), 1, "its Wednesday stays")
		return nil
	}))
}
