package compose_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// tableSchool is a school whose bookings a test step replaces: the state
// after each approval of the school.
type tableSchool struct {
	changedSchool
	bookings *[]carecontract.Booking
}

func (s tableSchool) Bookings(context.Context, []int64) (map[int64][]carecontract.Booking, error) {
	return map[int64][]carecontract.Booking{s.child: *s.bookings}, nil
}

// tableVersion is a stored version of the fixture: status, start and the
// weekdays of the short module (until 14:00).
type tableVersion struct {
	status, from string
	days         []string
}

// tableFixture is one Träger with one active contract and its run.
type tableFixture struct {
	db           *bun.DB
	ctx          context.Context
	organization int64
	contract     int64
	short, long  int64
	bookings     *[]carecontract.Booking
	conclusion   carecontract.Conclusion
}

func newTableFixture(t *testing.T, fixedMonths int, versions []tableVersion) *tableFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Zeitleiste", Slug: fmt.Sprintf("traeger-zeitleiste-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, db, organization)
	bookings := &[]carecontract.Booking{}
	school := tableSchool{changedSchool: changedSchool{school: carecontract.School{ID: testpkg.UniqueTestTenantID(t), Name: "OGS"},
		child: testpkg.UniqueTestTenantID(t), offering: testpkg.UniqueTestTenantID(t)}, bookings: bookings}
	f := &tableFixture{db: db, ctx: ctx, organization: organization, short: school.offering, long: school.offering + 1,
		bookings: bookings}
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id,
			status, sent_at, concluded_at, conclusion_method) VALUES (?, ?, ?, 'aktiv', NOW(), NOW(), 'mit_anmeldung') RETURNING id`,
		organization, school.school.ID, school.child).Scan(ctx, &f.contract))
	term, months, ends := "open_ended", any(nil), any(nil)
	if fixedMonths > 0 {
		term, months, ends = "fixed_months", fixedMonths, fixedEnd(fixedMonths)
	}
	for i, version := range versions {
		var id int64
		status := version.status
		if status == "abgeloest" {
			status = "wirksam" // stored wirksam first, superseded by the next one
		}
		require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
				status, effective_from, ends_on, child_first_name, child_last_name, school_name, term_kind, term_months, notice_months,
				notice_to, special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner, source)
			VALUES (?, ?, ?, ?, ?, ?, 'Lina', 'Muster', 'OGS', ?, ?, 1, 'month_end', false, '', false, '', '', '', ?) RETURNING id`,
			organization, f.contract, i+1, status, version.from, ends, term, months, sourceOf(i)).Scan(ctx, &id))
		for _, day := range version.days {
			_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
					weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, ?, ?, 'Kurzbetreuung', '14:00')`,
				organization, id, day, f.short)
			require.NoError(t, err)
		}
		if version.status == "abgeloest" {
			_, err := db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'abgeloest' WHERE id = ?`, id)
			require.NoError(t, err)
		}
	}
	f.conclusion = compose.NewConclusion(
		compose.ContractPorts{Letterheads: letterheadDouble{}, Post: &postDouble{}, Runtime: runtimeDouble{}, Access: school},
		compose.IntakePorts{Schools: school, Applications: school, Bookings: school, Access: school, Runtime: runtimeDouble{}, Modes: modesDouble{}})
	return f
}

func sourceOf(index int) string {
	if index == 0 {
		return "abschluss"
	}
	return "buchungsaenderung"
}

// fixedEnd is the last day of a fixed term from 1 August 2099.
func fixedEnd(months int) calendar.Date {
	return calendar.NewDate(2099, time.Month(8+months), 1).AddDays(-1)
}

// step sets the bookings after an approval and runs the amendment as the
// Worker does, again and again, until a run changes nothing: the timeline
// it settles on is what the billing reads. It answers every new version.
func (f *tableFixture) step(t *testing.T, bookings ...carecontract.Booking) carecontract.AmendmentRun {
	t.Helper()
	*f.bookings = bookings
	all := carecontract.AmendmentRun{}
	for range 10 {
		run, err := f.conclusion.AmendFromBookings(f.ctx, f.organization)
		require.NoError(t, err)
		if len(run.Amended) == 0 {
			return all
		}
		all.Amended = append(all.Amended, run.Amended...)
	}
	t.Fatal("the amendment never settles")
	return all
}

// inForce reads the versions in force between from and to.
func (f *tableFixture) inForce(t *testing.T, from, to calendar.Date) []carecontract.VersionPeriod {
	t.Helper()
	var periods []carecontract.VersionPeriod
	require.NoError(t, tenant.WithinTraeger(f.ctx, f.organization, func(txCtx context.Context) error {
		var err error
		periods, err = compose.NewContractVersions().VersionsInForce(txCtx, f.organization, from, to, f.contract)
		return err
	}))
	return periods
}

// assertOneVersionPerDay is rule 5 (#4681): VersionsInForce answers exactly
// one version for every day from from to to, without gap and without
// overlap. It answers the version of each day.
func (f *tableFixture) assertOneVersionPerDay(t *testing.T, from, to calendar.Date) map[calendar.Date]carecontract.Version {
	t.Helper()
	byDay := map[calendar.Date]carecontract.Version{}
	periods := f.inForce(t, from, to)
	for day := from; !day.After(to); day = day.AddDays(1) {
		holding := 0
		for _, period := range periods {
			if !day.Before(period.From) && !day.After(period.Until) {
				holding++
				byDay[day] = period.Version
			}
		}
		require.Equal(t, 1, holding, "exactly one version on %s", day)
	}
	return byDay
}

// on names what a version books on a day: "offering@end" per module.
func (f *tableFixture) on(version carecontract.Version, day calendar.Date) []string {
	names := []string{}
	for _, service := range version.ServicesOn(day) {
		module := "kurz"
		if service.CareOfferingID == f.long {
			module = "lang"
		}
		names = append(names, module+"@"+service.EndsAt)
	}
	sort.Strings(names)
	return names
}

// statuses lists number:status of every version, by number.
func (f *tableFixture) statuses(t *testing.T) []string {
	t.Helper()
	var rows []struct {
		Number int    `bun:"version_number"`
		Status string `bun:"status"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT version_number, status FROM care_contract.contract_versions
		WHERE contract_id = ? ORDER BY version_number`, f.contract).Scan(f.ctx, &rows))
	result := make([]string, len(rows))
	for i, row := range rows {
		result[i] = fmt.Sprintf("%d:%s", row.Number, row.Status)
	}
	return result
}

// The timeline of the versions (#4681, PR body "Zeitleiste der Versionen"):
// every case runs the amendment after each approval, checks with the shared
// helper that every day has exactly one version, and reads the booked care
// of the days that matter through VersionsInForce and ServicesOn.
func TestTimelineOfVersionsKeepsEveryApprovalAndOneVersionPerDay(t *testing.T) {
	t.Parallel()
	aug, feb, mar, apr := calendar.NewDate(2099, 8, 1), calendar.NewDate(2100, 2, 1), calendar.NewDate(2100, 3, 1), calendar.NewDate(2100, 4, 30)
	booking := func(offering int64, days []string, from, until *calendar.Date) carecontract.Booking {
		return carecontract.Booking{CareOfferingID: offering, Days: days, ValidFrom: from, ValidUntil: until}
	}
	planned := []tableVersion{{"abgeloest", "2099-08-01", []string{"mon"}}, {"wirksam", "2100-03-01", []string{"mon", "wed"}}}

	t.Run("D before E: the change holds until the planned version, which stays (rules 1, 2, 4)", func(t *testing.T) {
		t.Parallel()
		f := newTableFixture(t, 0, planned)
		f.step(t, booking(f.short, []string{"mon"}, &aug, &mar), booking(f.short, []string{"mon", "wed"}, &mar, nil),
			booking(f.long, []string{"fri"}, &feb, nil))
		byDay := f.assertOneVersionPerDay(t, aug, apr)
		assert.Equal(t, []string{"1:abgeloest", "2:wirksam", "3:abgeloest"}, f.statuses(t))
		assert.Equal(t, []string{"lang@16:30"}, f.on(byDay[calendar.NewDate(2100, 2, 5)], calendar.NewDate(2100, 2, 5)), "February Friday: the change")
		assert.Equal(t, []string{"kurz@14:00"}, f.on(byDay[calendar.NewDate(2100, 2, 8)], calendar.NewDate(2100, 2, 8)), "February Monday: as before")
		assert.Empty(t, f.on(byDay[calendar.NewDate(2100, 3, 5)], calendar.NewDate(2100, 3, 5)), "March Friday: the planned version does not take it over")
		assert.Equal(t, []string{"kurz@14:00"}, f.on(byDay[calendar.NewDate(2100, 3, 3)], calendar.NewDate(2100, 3, 3)), "March Wednesday: as planned")
	})

	t.Run("D equals E: the planned version is replaced cumulatively (rule 3)", func(t *testing.T) {
		t.Parallel()
		f := newTableFixture(t, 0, planned)
		f.step(t, booking(f.short, []string{"mon"}, &aug, &mar), booking(f.short, []string{"mon", "wed"}, &mar, nil),
			booking(f.long, []string{"fri"}, &mar, nil))
		byDay := f.assertOneVersionPerDay(t, aug, apr)
		assert.Equal(t, []string{"1:abgeloest", "2:ersetzt", "3:wirksam"}, f.statuses(t), "the replaced version stays visible")
		assert.Equal(t, []string{"lang@16:30"}, f.on(byDay[calendar.NewDate(2100, 3, 5)], calendar.NewDate(2100, 3, 5)), "March Friday: the new change")
		assert.Equal(t, []string{"kurz@14:00"}, f.on(byDay[calendar.NewDate(2100, 3, 3)], calendar.NewDate(2100, 3, 3)), "March Wednesday: from the planned version")
		assert.Equal(t, 3, byDay[calendar.NewDate(2100, 3, 3)].Number)
	})

	t.Run("the same D twice: the second approval replaces the first version cumulatively (rule 3)", func(t *testing.T) {
		t.Parallel()
		f := newTableFixture(t, 0, []tableVersion{{"wirksam", "2099-08-01", []string{"mon"}}})
		run := f.step(t, booking(f.short, []string{"mon"}, &aug, &feb), booking(f.long, []string{"mon"}, &feb, nil))
		require.Len(t, run.Amended, 1)
		f.assertOneVersionPerDay(t, aug, apr)
		run = f.step(t, booking(f.short, []string{"mon"}, &aug, &feb), booking(f.long, []string{"mon", "thu"}, &feb, nil))
		require.Len(t, run.Amended, 1)
		byDay := f.assertOneVersionPerDay(t, aug, apr)
		assert.Equal(t, []string{"1:abgeloest", "2:ersetzt", "3:wirksam"}, f.statuses(t))
		assert.Equal(t, []string{"lang@16:30"}, f.on(byDay[calendar.NewDate(2100, 2, 8)], calendar.NewDate(2100, 2, 8)), "Monday from the first approval")
		assert.Equal(t, []string{"lang@16:30"}, f.on(byDay[calendar.NewDate(2100, 2, 4)], calendar.NewDate(2100, 2, 4)), "Thursday from the second")
		again := f.step(t, booking(f.short, []string{"mon"}, &aug, &feb), booking(f.long, []string{"mon", "thu"}, &feb, nil))
		assert.Empty(t, again.Amended, "a repeated run changes nothing")
	})

	t.Run("D before all versions: a version from D until the first one (rules 1, 4)", func(t *testing.T) {
		t.Parallel()
		jun := calendar.NewDate(2099, 6, 1)
		f := newTableFixture(t, 0, []tableVersion{{"wirksam", "2099-08-01", []string{"mon"}}})
		f.step(t, booking(f.short, []string{"mon"}, &jun, nil))
		byDay := f.assertOneVersionPerDay(t, jun, apr)
		assert.Equal(t, []string{"1:wirksam", "2:abgeloest"}, f.statuses(t))
		assert.Equal(t, []string{"kurz@14:00"}, f.on(byDay[jun], jun), "June Monday: the earlier start")
		assert.Equal(t, 1, byDay[aug].Number, "from August the first version as before")
	})

	t.Run("D after the end of the contract: no version, and no contract on those days", func(t *testing.T) {
		t.Parallel()
		f := newTableFixture(t, 6, []tableVersion{{"wirksam", "2099-08-01", []string{"mon"}}})
		run := f.step(t, booking(f.short, []string{"mon"}, &aug, nil), booking(f.long, []string{"fri"}, &mar, nil))
		assert.Empty(t, run.Amended)
		assert.Equal(t, []string{"1:wirksam"}, f.statuses(t))
		f.assertOneVersionPerDay(t, aug, fixedEnd(6))
		assert.Empty(t, f.inForce(t, mar, apr), "after the fixed term the contract holds on no day")
	})
}
