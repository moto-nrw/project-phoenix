package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Explicit organization filters, not RLS, must protect the period, week,
// model and its history. An authenticated A transaction tries B's IDs while
// executing as the table owner (BYPASSRLS).
func TestHolidayTimeEditsKeepForeignOrganizationUntouchedBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Zeitfilter", Slug: fmt.Sprintf("time-filter-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	foreign := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Fremder Träger", Slug: fmt.Sprintf("time-foreign-%d", testpkg.UniqueTestTenantID(t)), Active: true}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, org)
	testpkg.OwnTraegerHolidayPeriods(t, db, foreign)
	school := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "Fremde OGS")
	monday := calendar.NewDate(2026, 6, 8)
	hostOwnChild(t, db, foreign, school, monday)
	var ids struct {
		Period int64
		Week   int64
		Model  int64
	}
	require.NoError(t, db.NewRaw(`SELECT week.holiday_period_id AS period, week.id AS week, model.id AS model
  FROM enrollment.holiday_period_weeks AS week JOIN enrollment.holiday_week_time_models AS model
  ON model.holiday_period_week_id = week.id WHERE week.organization_id = ?`, foreign).Scan(context.Background(), &ids))
	input := enrollment.HolidayPeriodInput{Name: "Fremd geändert", RegistrationOpensOn: calendar.NewDate(2026, 1, 1), RegistrationClosesOn: calendar.NewDate(2026, 6, 1),
		Weeks: []enrollment.HolidayWeekInput{{ID: ids.Week, StartsOn: monday, EndsOn: monday.AddDays(4), HostSchoolID: school,
			TimeModels: []enrollment.HolidayTimeModelInput{{ID: ids.Model, Name: "Geändert", StartsAt: "09:00", EndsAt: "13:00", PriceCents: 4500}}}}}
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, tenant.WithinTraeger(ctx, org, func(ctx context.Context) error {
		if err := testpkg.WithoutRequestRole(ctx); err != nil {
			return err
		}
		return postgres.NewHolidayPeriodStore().UpdateHolidayPeriod(ctx, org, ids.Period, input, monday.AddDays(2))
	}))
	var actual struct {
		Name    string
		Start   string
		End     string
		History string
	}
	require.NoError(t, db.NewRaw(`SELECT period.name, to_char(model.starts_at,'HH24:MI') AS start,
  to_char(model.ends_at,'HH24:MI') AS end, model.care_time_history::text AS history
  FROM enrollment.holiday_periods AS period JOIN enrollment.holiday_period_weeks AS week ON week.holiday_period_id = period.id
  JOIN enrollment.holiday_week_time_models AS model ON model.holiday_period_week_id = week.id WHERE model.id = ?`, ids.Model).Scan(context.Background(), &actual))
	assert.Equal(t, "Sommerferien", actual.Name)
	assert.Equal(t, "08:00", actual.Start)
	assert.Equal(t, "16:00", actual.End)
	assert.JSONEq(t, `{}`, actual.History)
}
