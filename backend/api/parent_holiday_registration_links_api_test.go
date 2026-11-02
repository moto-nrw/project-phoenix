package api

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// TestParentHolidayRegistrationLinksNameTheLinksTraegerOnly pins the root
// binding of the parents portal prefill (#4662, ADR 0113): a published link
// names its Träger and that Träger's schools, never another Träger's; a draft
// and an unknown link answer the portal's not-found sentinel.
func TestParentHolidayRegistrationLinksNameTheLinksTraegerOnly(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Vorbefüllung", identityaccess.TraegerRoleAdministration)
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vorbefüllung Fremd", identityaccess.TraegerRoleAdministration)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: f.db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	links := newParentHolidayRegistrationLinks(organizations, delivery.Module, testHealthCipher(), slog.Default())
	resolve := func(token string) (int64, []int64, error) {
		var organizationID int64
		var schoolIDs []int64
		err := tenant.WithinAdmin(tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db)), func(ctx context.Context) error {
			var resolveErr error
			organizationID, schoolIDs, resolveErr = links.ResolveHolidayRegistrationSchools(ctx, token)
			return resolveErr
		})
		return organizationID, schoolIDs, err
	}

	period := openHolidays(t, f, office)
	organizationID, schoolIDs, err := resolve(period.PublicToken)
	require.NoError(t, err)
	assert.Equal(t, office.organization, organizationID)
	assert.ElementsMatch(t, office.schools, schoolIDs)
	for _, school := range other.schools {
		assert.NotContains(t, schoolIDs, school, "no school of another Träger")
	}

	draft := createHolidayPeriod(t, f.traeger, other, januaryHolidays(other))
	rec := traegerJSON(t, f.traeger, other.token, http.MethodGet, holidayPeriodPath(draft.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, _, err = resolve(decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes()).PublicToken)
	assert.ErrorIs(t, err, parentAPI.ErrHolidayRegistrationLinkNotFound, "a draft has no open link")
	_, _, err = resolve("unknown-token")
	assert.ErrorIs(t, err, parentAPI.ErrHolidayRegistrationLinkNotFound)
}
