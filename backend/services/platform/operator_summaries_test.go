package platform_test

import (
	"context"
	"errors"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSummariesRepo lets the service tests drive each repo method's return
// value without spinning up a real database. The service is a thin
// admin-tx wrapper around the repo, so behavior verification reduces to
// asserting the right repo method is called and the right error/payload
// path is taken.
type mockSummariesRepo struct {
	statsFn      func(context.Context) (*platformModels.ProvisioningStats, error)
	orgsFn       func(context.Context) ([]*platformModels.OrganizationSummary, error)
	schoolsFn    func(context.Context) ([]*platformModels.SchoolSummary, error)
	schoolsByOrg func(context.Context, int64) ([]*platformModels.SchoolSummary, error)
	personsBySch func(context.Context, int64) ([]platformModels.OperatorPersonInfo, error)
	personsByOrg func(context.Context, int64) ([]platformModels.OperatorPersonInfo, error)
	devicesFn    func(context.Context, platformModels.OperatorDeviceFilter) ([]platformModels.OperatorDeviceRow, error)
	pwaUsageFn   func(context.Context, int64, time.Duration) ([]platformModels.SchoolPWAUsageRow, error)
}

func (m *mockSummariesRepo) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]platformModels.SchoolPWAUsageRow, error) {
	if m.pwaUsageFn != nil {
		return m.pwaUsageFn(ctx, tenantID, window)
	}
	return nil, nil
}

func (m *mockSummariesRepo) Stats(ctx context.Context) (*platformModels.ProvisioningStats, error) {
	if m.statsFn != nil {
		return m.statsFn(ctx)
	}
	return &platformModels.ProvisioningStats{}, nil
}

func (m *mockSummariesRepo) OrganizationSummaries(ctx context.Context) ([]*platformModels.OrganizationSummary, error) {
	if m.orgsFn != nil {
		return m.orgsFn(ctx)
	}
	return nil, nil
}

func (m *mockSummariesRepo) SchoolSummaries(ctx context.Context) ([]*platformModels.SchoolSummary, error) {
	if m.schoolsFn != nil {
		return m.schoolsFn(ctx)
	}
	return nil, nil
}

func (m *mockSummariesRepo) SchoolSummariesByOrganization(ctx context.Context, organizationID int64) ([]*platformModels.SchoolSummary, error) {
	if m.schoolsByOrg != nil {
		return m.schoolsByOrg(ctx, organizationID)
	}
	return nil, nil
}

func (m *mockSummariesRepo) PersonsBySchool(ctx context.Context, schoolID int64) ([]platformModels.OperatorPersonInfo, error) {
	if m.personsBySch != nil {
		return m.personsBySch(ctx, schoolID)
	}
	return nil, nil
}

func (m *mockSummariesRepo) PersonsByOrganization(ctx context.Context, organizationID int64) ([]platformModels.OperatorPersonInfo, error) {
	if m.personsByOrg != nil {
		return m.personsByOrg(ctx, organizationID)
	}
	return nil, nil
}

func TestOperatorProvisioningService_GetProvisioningStats(t *testing.T) {
	t.Parallel()

	expected := &platformModels.ProvisioningStats{
		TraegerCount: 3,
		SchulenCount: 7,
		KontenCount:  12,
		GeraeteCount: 4,
	}
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{
			statsFn: func(context.Context) (*platformModels.ProvisioningStats, error) {
				return expected, nil
			},
		},
	})

	got, err := service.GetProvisioningStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestOperatorProvisioningService_GetProvisioningStats_RepoError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("db boom")
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{
			statsFn: func(context.Context) (*platformModels.ProvisioningStats, error) {
				return nil, repoErr
			},
		},
	})

	got, err := service.GetProvisioningStats(context.Background())
	require.Nil(t, got)
	require.ErrorIs(t, err, repoErr)
}

func TestOperatorProvisioningService_ListOrganizationSummaries(t *testing.T) {
	t.Parallel()

	expected := []*platformModels.OrganizationSummary{
		{ID: 1, Name: "Org A", Slug: "org-a", Active: true, SchulenCount: 2, KontenCount: 5},
	}
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{
			orgsFn: func(context.Context) ([]*platformModels.OrganizationSummary, error) {
				return expected, nil
			},
		},
	})

	got, err := service.ListOrganizationSummaries(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestOperatorProvisioningService_ListSchoolSummaries(t *testing.T) {
	t.Parallel()

	expected := []*platformModels.SchoolSummary{
		{ID: 11, OrganizationID: 1, OrganizationName: "Org A", Name: "School 1", Slug: "school-1", Subdomain: "s1", Active: true, KontenCount: 4},
	}
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		SummariesRepo: &mockSummariesRepo{
			schoolsFn: func(context.Context) ([]*platformModels.SchoolSummary, error) {
				return expected, nil
			},
		},
	})

	got, err := service.ListSchoolSummaries(context.Background())
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestOperatorProvisioningService_ListOrganizationSchoolSummaries(t *testing.T) {
	t.Parallel()

	expected := []*platformModels.SchoolSummary{
		{ID: 21, OrganizationID: 9, Name: "School", Slug: "school", Subdomain: "school", Active: true},
	}
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(_ context.Context, id int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SummariesRepo: &mockSummariesRepo{
			schoolsByOrg: func(_ context.Context, orgID int64) ([]*platformModels.SchoolSummary, error) {
				require.Equal(t, int64(9), orgID, "service must forward the org id verbatim")
				return expected, nil
			},
		},
	})

	got, err := service.ListOrganizationSchoolSummaries(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestOperatorProvisioningService_ListOrganizationSchoolSummaries_NotFound(t *testing.T) {
	t.Parallel()

	// Deliberate guard: service must return OrganizationNotFoundError BEFORE
	// invoking the summaries repo, so a missing org never produces a leaky
	// empty list that the operator UI would render as "this org has no schools".
	repoCalled := false
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return nil, nil
			},
		},
		SummariesRepo: &mockSummariesRepo{
			schoolsByOrg: func(context.Context, int64) ([]*platformModels.SchoolSummary, error) {
				repoCalled = true
				return nil, nil
			},
		},
	})

	got, err := service.ListOrganizationSchoolSummaries(context.Background(), 999)
	require.Nil(t, got)
	require.Error(t, err)
	var notFound *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, int64(999), notFound.OrganizationID)
	assert.False(t, repoCalled, "summaries repo must not be invoked for a missing organization")
}

func TestOperatorProvisioningService_ListOrganizationPersons(t *testing.T) {
	t.Parallel()

	expected := []platformModels.OperatorPersonInfo{
		{ID: 101, FirstName: "Anna", LastName: "Schmidt", SchoolID: 11, OrganizationID: 9, OrganizationName: "Org"},
	}
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return &platformModels.Organization{Name: "Org", Slug: "org", Active: true}, nil
			},
		},
		SummariesRepo: &mockSummariesRepo{
			personsByOrg: func(_ context.Context, orgID int64) ([]platformModels.OperatorPersonInfo, error) {
				require.Equal(t, int64(9), orgID)
				return expected, nil
			},
		},
	})

	got, err := service.ListOrganizationPersons(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestOperatorProvisioningService_ListOrganizationPersons_NotFound(t *testing.T) {
	t.Parallel()

	repoCalled := false
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return nil, nil
			},
		},
		SummariesRepo: &mockSummariesRepo{
			personsByOrg: func(context.Context, int64) ([]platformModels.OperatorPersonInfo, error) {
				repoCalled = true
				return nil, nil
			},
		},
	})

	got, err := service.ListOrganizationPersons(context.Background(), 7)
	require.Nil(t, got)
	require.Error(t, err)
	var notFound *platformSvc.OrganizationNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, int64(7), notFound.OrganizationID)
	assert.False(t, repoCalled, "summaries repo must not be invoked for a missing organization")
}

func TestOperatorProvisioningService_ListOrganizationPersons_OrgRepoError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("org lookup failed")
	service := newTestOperatorProvisioningService(t, platformSvc.OperatorProvisioningServiceConfig{
		OrganizationRepo: &mockOrganizationRepo{
			findByIDFn: func(context.Context, int64) (*platformModels.Organization, error) {
				return nil, repoErr
			},
		},
		SummariesRepo: &mockSummariesRepo{},
	})

	got, err := service.ListOrganizationPersons(context.Background(), 1)
	require.Nil(t, got)
	require.ErrorIs(t, err, repoErr)
}
