package platform_test

import (
	"context"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorProvisioningService_GetSchoolPWAUsage(t *testing.T) {
	school := &platformModels.School{}
	school.ID = 7

	t.Run("maps portal rows into the response shape", func(t *testing.T) {
		var gotTenantID int64
		var gotWindow time.Duration
		service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
			SummariesRepo: &mockSummariesRepo{
				pwaUsageFn: func(_ context.Context, tenantID int64, window time.Duration) ([]platformModels.SchoolPWAUsageRow, error) {
					gotTenantID = tenantID
					gotWindow = window
					return []platformModels.SchoolPWAUsageRow{
						{TenantID: 7, Portal: "staff", StandaloneUsers: 3, EligibleUsers: 12},
						{TenantID: 7, Portal: "parent", StandaloneUsers: 47, EligibleUsers: 210},
					}, nil
				},
			},
			SchoolRepo: &testpkg.SchoolRepoMock{
				FindByIDFn: func(context.Context, int64) (*platformModels.School, error) { return school, nil },
			},
		})

		usage, err := service.GetSchoolPWAUsage(context.Background(), school.ID)
		require.NoError(t, err)
		assert.Equal(t, school.ID, gotTenantID)
		assert.Equal(t, 30*24*time.Hour, gotWindow)
		assert.Equal(t, 30, usage.WindowDays)
		assert.Equal(t, platformSvc.PWAPortalUsage{StandaloneUsers: 3, EligibleUsers: 12}, usage.Staff)
		assert.Equal(t, platformSvc.PWAPortalUsage{StandaloneUsers: 47, EligibleUsers: 210}, usage.Parent)
	})

	t.Run("missing buckets stay zero", func(t *testing.T) {
		service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
			SummariesRepo: &mockSummariesRepo{},
			SchoolRepo: &testpkg.SchoolRepoMock{
				FindByIDFn: func(context.Context, int64) (*platformModels.School, error) { return school, nil },
			},
		})
		usage, err := service.GetSchoolPWAUsage(context.Background(), school.ID)
		require.NoError(t, err)
		assert.Equal(t, platformSvc.PWAPortalUsage{}, usage.Staff)
		assert.Equal(t, platformSvc.PWAPortalUsage{}, usage.Parent)
	})

	t.Run("unknown school returns SchoolNotFoundError", func(t *testing.T) {
		service := platformSvc.NewOperatorProvisioningService(platformSvc.OperatorProvisioningServiceConfig{
			SummariesRepo: &mockSummariesRepo{},
			SchoolRepo:    &testpkg.SchoolRepoMock{},
		})
		_, err := service.GetSchoolPWAUsage(context.Background(), 999)
		require.Error(t, err)
		var notFoundErr *platformSvc.SchoolNotFoundError
		require.ErrorAs(t, err, &notFoundErr)
	})
}
