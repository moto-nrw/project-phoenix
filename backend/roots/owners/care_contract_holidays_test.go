//go:build testhooks

package owners

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Betreuungsvertrag asks inside its Träger transaction; the binding
// reads each school's federal state in a unit of its own (ADR 0175) and
// answers School Calendar's holidays of that state for that school
// (#6350). The read runs as the administrative role, beneath row security,
// so only the binding keeps the schools apart.
func TestCareContractSchoolHolidaysFollowEachSchoolsFederalState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database, unit := testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)
	suffix := testpkg.UniqueTestTenantID(t)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Feiertage", Slug: fmt.Sprintf("feiertage-%d", suffix), Active: true}).ID
	nrw := testpkg.CreateTestSchoolInOrganization(t, db, org, "OGS Düsseldorf")
	berlin := testpkg.CreateTestSchoolInOrganization(t, db, org, "OGS Berlin")
	unknown := testpkg.CreateTestSchoolInOrganization(t, db, org, "OGS ohne Land")
	testpkg.SetTestTenantSetting(t, db, berlin, configModels.KeyFederalState, `"DE-BE"`)
	testpkg.SetTestTenantSetting(t, db, unknown, configModels.KeyFederalState, `"DE-XX"`)

	settings, err := NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	records, err := BuildSchoolCalendarRecords(database, func(schoolCalendarCompose.Observation) {}, settings.Settings)
	require.NoError(t, err)
	holidays := NewCareContractSchoolHolidays(settings.Settings, records)
	ctx := tenant.WithUnitOfWork(context.Background(), unit)
	from, to := calendar.NewDate(2026, 5, 1), calendar.NewDate(2026, 6, 30)

	var days map[int64]map[calendar.Date]bool
	require.NoError(t, testpkg.WithinTraeger(ctx, org, func(txCtx context.Context) error {
		days, err = holidays.SchoolHolidayDates(txCtx, []int64{nrw, berlin}, from, to)
		return err
	}))
	corpusChristi, ascension := calendar.NewDate(2026, 6, 4), calendar.NewDate(2026, 5, 14)
	assert.True(t, days[nrw][corpusChristi], "Fronleichnam is a holiday in Nordrhein-Westfalen, the default state")
	assert.False(t, days[berlin][corpusChristi], "and none in Berlin")
	assert.True(t, days[nrw][ascension], "Christi Himmelfahrt is one everywhere")
	assert.True(t, days[berlin][ascension], "Christi Himmelfahrt is one everywhere")
	assert.False(t, days[nrw][calendar.NewDate(2026, 7, 1)], "nothing outside the window")

	err = testpkg.WithinTraeger(ctx, org, func(txCtx context.Context) error {
		_, err := holidays.SchoolHolidayDates(txCtx, []int64{nrw, unknown}, from, to)
		return err
	})
	assert.ErrorIs(t, err, schoolcalendar.ErrFederalStateUnavailable, "an unknown state is no silent default")
}
