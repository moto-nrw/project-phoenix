package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestSubstitutionCapabilityKeepsTheExternalMarker pins #3823 and #4573: the
// picker of a running supervision labels external caregivers with their
// account status and organization, so the workforce facade must pass them
// through instead of rebuilding the reference.
func TestSubstitutionCapabilityKeepsTheExternalMarker(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	caregiver := testpkg.CreateTestStaff(t, db, "Interne", "Kraft")
	external := testpkg.CreateTestExternalCaregiverRecord(t, db, "Trommeln").Staff
	capability := api.SubstitutionCapability(substitutionContract{overview: func(context.Context, education.Caller, education.OverviewQuery) (*education.OverviewResult, error) {
		return &education.OverviewResult{RunningSupervisions: []education.RunningSupervision{{
			ID: caregiver.ID, AvailableTargets: []education.StaffRef{
				{ID: caregiver.ID, FullName: "Interne Kraft"},
				{ID: external.ID, FullName: "Extern Kursleitung", IsExternal: true, ExternalAccountStatus: "invitation_pending", ExternalOrganization: "Musikschule"},
			},
		}}}, nil
	}})

	out, err := capability.Overview(testpkg.Ctx(t), workforce.SubstitutionCaller{TenantID: testpkg.Tenant(t), HasPermission: func(string) bool { return true }}, workforce.SubstitutionOverviewQuery{IncludeTargets: true})
	require.NoError(t, err)
	require.Equal(t, []workforce.StaffRef{
		{ID: caregiver.ID, FullName: "Interne Kraft"},
		{ID: external.ID, FullName: "Extern Kursleitung", IsExternal: true, ExternalAccountStatus: "invitation_pending", ExternalOrganization: "Musikschule"},
	}, out.RunningSupervisions[0].AvailableTargets)
}
