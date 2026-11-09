package compose_test

import (
	"bytes"
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

// changedSchool is a school whose accepted child booked the module until
// 14:00 on Monday; the school approved the parents' request to switch to the
// module until 16:30 on Monday and Tuesday from 1 March 2100.
type changedSchool struct {
	school   carecontract.School
	child    int64
	offering int64
}

func (s changedSchool) TraegerSchools(context.Context, int64) ([]carecontract.School, error) {
	return []carecontract.School{s.school}, nil
}

func (s changedSchool) AcceptedApplications(context.Context, []int64) ([]carecontract.Application, error) {
	return []carecontract.Application{{RequestChildID: s.child, ChildFirstName: "Lina", ChildLastName: "Muster"}}, nil
}

func (s changedSchool) NamePhases(_ context.Context, offerings []carecontract.Offering) ([]carecontract.Offering, error) {
	return offerings, nil
}

func (s changedSchool) Bookings(context.Context, []int64) (map[int64][]carecontract.Booking, error) {
	from, switched := calendar.NewDate(2099, 8, 1), calendar.NewDate(2100, 3, 1)
	return map[int64][]carecontract.Booking{s.child: {
		{CareOfferingID: s.offering, Days: []string{"mon"}, ValidFrom: &from, ValidUntil: &switched},
		{CareOfferingID: s.offering + 1, Days: []string{"mon", "tue"}, ValidFrom: &switched},
	}}, nil
}

func (s changedSchool) Offerings(context.Context, []int64) ([]carecontract.Offering, error) {
	days := func(end string) map[string]string {
		return map[string]string{"mon": end, "tue": end, "wed": end, "thu": end, "fri": end}
	}
	return []carecontract.Offering{
		{ID: s.offering, Name: "Kurzbetreuung", Active: true, EndTimes: days("14:00")},
		{ID: s.offering + 1, Name: "Ganztag", Active: true, EndTimes: days("16:30")},
	}, nil
}

func (s changedSchool) RecordAccess(context.Context, carecontract.SchoolAccess) error { return nil }

// activeContract stores a concluded contract whose first version books the
// module until 14:00 on Monday from 1 August 2099. As the intake makes it,
// the version ends the day before the module's booking ends.
func activeContract(t *testing.T, ctx context.Context, organization int64, school changedSchool) int64 {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	var contract, version int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
			status, sent_at, concluded_at, conclusion_method) VALUES (?, ?, ?, 'aktiv', NOW(), NOW(), 'mit_anmeldung') RETURNING id`,
		organization, school.school.ID, school.child).Scan(ctx, &contract))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, ends_on, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, 'wirksam', '2099-08-01', '2100-02-28', 'Lina', 'Muster', 'OGS Am Lindenhof', 'open_ended', 1, 'month_end',
			false, '', false, '', '', '') RETURNING id`, organization, contract).Scan(ctx, &version))
	_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
			weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, 'mon', ?, 'Kurzbetreuung', '14:00')`,
		organization, version, school.offering)
	require.NoError(t, err)
	return contract
}

func TestPublicContractFollowsAnApprovedChangeWithANewVersionAndAnswersOneVersionPerDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	newTraeger := func(name string) int64 {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-aenderung-%d", testpkg.UniqueTestTenantID(t)),
		}).ID
		testpkg.OwnTraegerCareContracts(t, db, organization)
		return organization
	}
	organization, foreign := newTraeger("Träger Änderung"), newTraeger("Träger Änderung Fremd")
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("aenderung-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())
	reviewer := carecontract.Reviewer{OrganizationID: organization, AccountID: account.ID}
	school := changedSchool{school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS Am Lindenhof"},
		child: testpkg.UniqueTestTenantID(t), offering: testpkg.UniqueTestTenantID(t)}
	contract := activeContract(t, ctx, organization, school)
	contractPorts := compose.ContractPorts{Holidays: holidaysDouble{}, Letterheads: letterheadDouble{}, Post: &postDouble{}, Runtime: runtimeDouble{}, Access: school,
		Ledger: ledgerDouble{}, Links: testSigningLinks()}
	amendments := compose.NewConclusion(contractPorts, compose.IntakePorts{Schools: school, Applications: school,
		Bookings: school, Access: school, Runtime: runtimeDouble{}, Modes: modesDouble{}, Mail: noMailBlocks{}, Letterheads: letterheadDouble{}})

	run, err := amendments.AmendFromBookings(ctx, organization)
	require.NoError(t, err)
	assert.Equal(t, []carecontract.AmendedContract{{ContractID: contract, VersionNumber: 2, EffectiveFrom: calendar.NewDate(2100, 3, 1)}},
		run.Amended, "the approved change becomes version 2 from its effective day")
	again, err := amendments.AmendFromBookings(ctx, organization)
	require.NoError(t, err)
	assert.Empty(t, again.Amended, "a repeated run makes no second version")

	contracts := compose.NewContracts(contractPorts)
	detail, err := contracts.Contract(ctx, reviewer, contract)
	require.NoError(t, err)
	require.Len(t, detail.Versions, 2, "the old version stays readable")
	first, second := detail.Versions[0], detail.Versions[1]
	assert.Equal(t, carecontract.VersionSuperseded, first.Status)
	require.NotNil(t, first.ValidUntil)
	assert.Equal(t, calendar.NewDate(2100, 2, 28), *first.ValidUntil, "the old version ends the day before the new one")
	assert.Equal(t, carecontract.VersionEffective, second.Status)
	assert.Equal(t, carecontract.SourceBookingChange, second.Source)
	assert.Nil(t, second.ValidUntil)
	assert.Equal(t, []carecontract.Service{
		{Weekday: "mon", CareOfferingID: school.offering + 1, ModuleName: "Ganztag", EndsAt: "16:30"},
		{Weekday: "tue", CareOfferingID: school.offering + 1, ModuleName: "Ganztag", EndsAt: "16:30"},
	}, second.Services)
	assert.Equal(t, 1, detail.Version.Number, "until March the first version is the one in force")

	pdf, err := contracts.VersionDocument(ctx, reviewer, contract, 2)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(pdf.Data, []byte("%PDF")))
	_, err = contracts.VersionDocument(ctx, reviewer, contract, 9)
	require.ErrorIs(t, err, carecontract.ErrVersionNotFound)

	versions := compose.NewContractVersions()
	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		periods, err := versions.VersionsInForce(txCtx, organization, calendar.NewDate(2100, 2, 1), calendar.NewDate(2100, 3, 31))
		if err != nil {
			return err
		}
		require.Len(t, periods, 2, "February and March each have exactly one version")
		assert.Equal(t, []calendar.Date{calendar.NewDate(2100, 2, 1), calendar.NewDate(2100, 2, 28)},
			[]calendar.Date{periods[0].From, periods[0].Until})
		assert.Equal(t, 1, periods[0].Version.Number)
		assert.Equal(t, []calendar.Date{calendar.NewDate(2100, 3, 1), calendar.NewDate(2100, 3, 31)},
			[]calendar.Date{periods[1].From, periods[1].Until})
		assert.Equal(t, 2, periods[1].Version.Number)
		monday := calendar.NewDate(2100, 3, 1)
		assert.Len(t, periods[1].Version.ServicesOn(monday), 1, "Monday books the long module")
		day, err := versions.VersionsInForce(txCtx, organization, monday, monday)
		require.Len(t, day, 1, "one version per contract and day")
		return err
	}))
	require.NoError(t, tenant.WithinTraeger(ctx, foreign, func(txCtx context.Context) error {
		periods, err := versions.VersionsInForce(txCtx, foreign, calendar.NewDate(2100, 2, 1), calendar.NewDate(2100, 3, 31))
		assert.Empty(t, periods, "no Träger reads another Träger's contracts")
		return err
	}))
}

type modesDouble struct{}

func (modesDouble) ConclusionMode(context.Context, int64) (string, error) {
	return carecontract.ModeSendLater, nil
}
