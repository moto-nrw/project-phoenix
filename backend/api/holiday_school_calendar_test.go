package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The holiday weeks ask inside their Träger transaction which days a host
// school gives no care (#4702): the statutory holidays of its own federal
// state, its own closing days and the closing days its Träger planned for it,
// one entry per date. The closing days are read in each school's own school
// transaction, so the answer for one school never holds another's.
func TestHolidaySchoolCalendarNamesEachHostSchoolsDaysWithoutCare(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database, unit := testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)
	suffix := testpkg.UniqueTestTenantID(t)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Ferientage", Slug: fmt.Sprintf("ferientage-%d", suffix), Active: true}).ID
	testpkg.OwnTraegerClosingDays(t, db, org)
	pforzheim := testpkg.CreateTestSchoolInOrganization(t, db, org, "OGS Pforzheim")
	iserlohn := testpkg.CreateTestSchoolInOrganization(t, db, org, "OGS Iserlohn")
	testpkg.SetTestTenantSetting(t, db, pforzheim, configModels.KeyFederalState, `"DE-BW"`)

	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	records, err := owners.BuildSchoolCalendarRecords(database, func(schoolCalendarCompose.Observation) {}, settings.Settings)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)

	_, err = records.CreateClosingDay(tenant.WithTenantID(ctx, iserlohn), schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2026-06-03", EndDate: "2026-06-05", Reason: "Pädagogische Tage",
	}})
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `INSERT INTO schedule.traeger_closing_days (organization_id, start_date, end_date, reason, school_ids)
		VALUES (?, '2026-05-15', '2026-05-15', 'Brückentag', ARRAY[?]::bigint[])`, org, pforzheim)
	require.NoError(t, err)

	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	source := owners.NewHolidaySchoolCalendar(settings.Settings, records, organizations)
	var days map[int64][]enrollment.HolidayNoCareDay
	require.NoError(t, testpkg.WithinTraeger(ctx, org, func(txCtx context.Context) error {
		days, err = source.HolidayNoCareDays(txCtx, []int64{pforzheim, iserlohn}, calendar.NewDate(2026, 5, 11), calendar.NewDate(2026, 6, 5))
		return err
	}))
	assert.Equal(t, []enrollment.HolidayNoCareDay{
		{Date: calendar.NewDate(2026, 5, 14), Kind: enrollment.HolidayNoCareHoliday, Name: "Christi Himmelfahrt"},
		{Date: calendar.NewDate(2026, 5, 15), Kind: enrollment.HolidayNoCareClosingDay, Name: "Brückentag"},
		{Date: calendar.NewDate(2026, 5, 25), Kind: enrollment.HolidayNoCareHoliday, Name: "Pfingstmontag"},
		{Date: calendar.NewDate(2026, 6, 4), Kind: enrollment.HolidayNoCareHoliday, Name: "Fronleichnam"},
	}, days[pforzheim], "Baden-Württemberg's holidays and the Träger's bridge day")
	assert.Equal(t, []enrollment.HolidayNoCareDay{
		{Date: calendar.NewDate(2026, 5, 14), Kind: enrollment.HolidayNoCareHoliday, Name: "Christi Himmelfahrt"},
		{Date: calendar.NewDate(2026, 5, 25), Kind: enrollment.HolidayNoCareHoliday, Name: "Pfingstmontag"},
		{Date: calendar.NewDate(2026, 6, 3), Kind: enrollment.HolidayNoCareClosingDay, Name: "Pädagogische Tage"},
		{Date: calendar.NewDate(2026, 6, 4), Kind: enrollment.HolidayNoCareHoliday, Name: "Fronleichnam"},
		{Date: calendar.NewDate(2026, 6, 5), Kind: enrollment.HolidayNoCareClosingDay, Name: "Pädagogische Tage"},
	}, days[iserlohn], "the school's own closing days, not the bridge day of the other school; a holiday names a day that is both")
	foreignOrg := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Fremde Ferientage", Slug: fmt.Sprintf("fremde-ferientage-%d", suffix), Active: true}).ID
	foreign := testpkg.CreateTestSchoolInOrganization(t, db, foreignOrg, "Fremde OGS")
	_, err = records.CreateClosingDay(tenant.WithTenantID(ctx, foreign), schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2026-06-03", EndDate: "2026-06-05", Reason: "Vertraulicher Schließgrund",
	}})
	require.NoError(t, err)
	testpkg.OwnTraegerHolidayPeriods(t, db, org)
	periods := enrollmentCompose.NewHolidayPeriods(nil, source)
	input := enrollment.HolidayPeriodInput{
		Name: "Sommer", RegistrationOpensOn: calendar.NewDate(2026, 1, 1), RegistrationClosesOn: calendar.NewDate(2026, 5, 1),
	}
	require.NoError(t, testpkg.WithinTraeger(ctx, org, func(txCtx context.Context) error {
		period, err := periods.CreateHolidayPeriod(txCtx, org, input)
		require.NoError(t, err)
		foreignInput := input
		foreignInput.Weeks = []enrollment.HolidayWeekInput{{HostSchoolID: foreign, StartsOn: calendar.NewDate(2026, 6, 3), EndsOn: calendar.NewDate(2026, 6, 5)}}
		_, err = periods.CreateHolidayPeriod(txCtx, org, foreignInput)
		assert.ErrorIs(t, err, enrollment.ErrHolidayHostSchoolOutsideTraeger, "create authorizes the host school before reading its closure")
		_, err = periods.UpdateHolidayPeriod(txCtx, org, period.ID, foreignInput)
		assert.ErrorIs(t, err, enrollment.ErrHolidayHostSchoolOutsideTraeger, "update authorizes the host school before reading its closure")
		return nil
	}))

}
