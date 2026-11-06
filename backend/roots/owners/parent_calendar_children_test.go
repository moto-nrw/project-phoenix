package owners

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type calendarChildLinksStub []ParentChildLink

func (s calendarChildLinksStub) ListByAccount(context.Context, int64) ([]ParentChildLink, error) {
	return append([]ParentChildLink(nil), s...), nil
}

// The parent calendar and its subscribed ICS feed show the children of the
// schools that booked Eltern-App only, like the parents portal (#4895). The
// calendar lists the children inside its administrative lookup, so the
// booking read joins that transaction instead of opening a tenant one.
func TestParentCalendarChildrenOfASchoolWithoutElternAppAreHidden(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	settings, err := NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	booked := testpkg.Tenant(t)
	cancelled, _ := testpkg.CreateTestTenant(t, db)
	testpkg.SetTestTenantSetting(t, db, cancelled, "module.eltern_app", "false")

	children := ParentCalendarChildren(calendarChildLinksStub{
		{StudentID: 6201, TenantID: booked},
		{StudentID: 6202, TenantID: cancelled},
	}, settings.Settings)

	var listed []ParentChildLink
	require.NoError(t, tenant.WithinAdmin(testpkg.Ctx(t), func(adminCtx context.Context) error {
		listed, err = children.ListByAccount(adminCtx, 7301)
		return err
	}))
	require.Len(t, listed, 1)
	assert.Equal(t, booked, listed[0].TenantID)
}
