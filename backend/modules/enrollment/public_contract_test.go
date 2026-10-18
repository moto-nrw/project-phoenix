package enrollment_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicEnrollmentContractRoundTripsPhaseDatesAndUpdates(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	module := compose.New()
	phase := &enrollment.Phase{Name: "Contract phase", ServiceStartDate: "2026-08-24", ServiceEndDate: "2027-07-31", IsActive: true}
	require.NoError(t, module.InsertPhase(testpkg.Ctx(t), phase))
	require.Positive(t, phase.ID)
	require.Equal(t, testpkg.Tenant(t), phase.TenantID)
	found, err := module.Phase(testpkg.Ctx(t), phase.ID)
	require.NoError(t, err)
	require.Equal(t, phase.Name, found.Name)
	require.Equal(t, phase.ServiceStartDate, found.ServiceStartDate)
	require.Equal(t, phase.ServiceEndDate, found.ServiceEndDate)
	found.Name = "Updated contract phase"
	require.NoError(t, module.UpdatePhase(testpkg.Ctx(t), found))
	rows, err := module.Phases(testpkg.Ctx(t))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, phase.ID, rows[0].ID)
	require.Equal(t, found.Name, rows[0].Name)
}
