package compose_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	calendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

type calendarChildLinksStub []calendarCompose.ChildSummary

func (s calendarChildLinksStub) ListByAccount(context.Context, int64) ([]*calendarCompose.ChildSummary, error) {
	children := make([]*calendarCompose.ChildSummary, len(s))
	for i := range s {
		children[i] = &s[i]
	}
	return children, nil
}

// schoolRecordingRuntime records the schools the parent calendar reads
// events in and reads none; the administrative lookup runs for real.
type schoolRecordingRuntime struct {
	*calendarCompose.TenantRuntime
	schools []int64
}

func (r *schoolRecordingRuntime) WithinTenant(_ context.Context, tenantID int64, _ func(context.Context) error) error {
	r.schools = append(r.schools, tenantID)
	return nil
}

// The parent calendar and its subscribed ICS feed show the children of the
// schools that booked Eltern-App only, like the parents portal (#4895). The
// calendar lists the children inside its administrative lookup, so the
// booking read joins that transaction instead of opening a tenant one.
func TestParentCalendarChildrenOfASchoolWithoutElternAppAreHidden(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	booked := testpkg.Tenant(t)
	cancelled, _ := testpkg.CreateTestTenant(t, db)
	testpkg.SetTestTenantSetting(t, db, cancelled, "module.eltern_app", "false")

	runtime := &schoolRecordingRuntime{TenantRuntime: calendarCompose.NewTenantRuntime(db)}
	cfg := owners.BindCalendarElternAppBookings(calendarCompose.Config{
		ChildRepo: calendarChildLinksStub{
			{StudentID: 6201, TenantID: booked},
			{StudentID: 6202, TenantID: cancelled},
		},
		Runtime: runtime,
	}, settings.Settings)

	_, err = owners.NewCalendarPortal(cfg).ListMyParentEvents(testpkg.Ctx(t), 7301, portalDate(2026, 1, 5), portalDate(2026, 1, 11))
	require.NoError(t, err)
	listed := runtime.schools
	require.Len(t, listed, 1)
	assert.Equal(t, booked, listed[0])
}
