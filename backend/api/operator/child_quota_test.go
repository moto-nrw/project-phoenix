package operator_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/operator"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type childQuotaProvisioning struct {
	organizationtenancy.Provisioning
	updateSchoolFn func(context.Context, int64, organizationtenancy.SchoolChanges, int64, net.IP) (*organizationtenancy.School, error)
}

func (s *childQuotaProvisioning) UpdateSchool(ctx context.Context, id int64, changes organizationtenancy.SchoolChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
	return s.updateSchoolFn(ctx, id, changes, operatorID, clientIP)
}

func childQuotaRequest(accessToken string) *http.Request {
	req := httptest.NewRequest(http.MethodPut, "/schools/10", bytes.NewBufferString(
		`{"organization_id":7,"name":"Schule","slug":"schule","subdomain":"schule","active":true,"child_quota":{"bundles":2}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return req
}

func childQuotaRouter(t *testing.T, provisioning organizationtenancy.Provisioning) http.Handler {
	t.Helper()
	authService := &mockOperatorAuthService{
		getOperatorFn: func(_ context.Context, id int64) (*identityaccess.Operator, error) {
			op := &identityaccess.Operator{Active: true}
			op.ID = id
			return op, nil
		},
	}
	return operator.NewResource(operator.ResourceConfig{
		AuthService:         authService,
		ProvisioningService: provisioning,
		Sessions:            operatorTestSessions(t, authService),
	}).Router()
}

// TestOperatorSetsTheKinderkontingentOnTheSchoolRoute pins that the existing
// school update route carries the Kinderkontingent (#3567) to the owner with
// the acting operator, who is recorded in the audit entry.
func TestOperatorSetsTheKinderkontingentOnTheSchoolRoute(t *testing.T) {
	t.Parallel()
	var got organizationtenancy.SchoolChanges
	var actor int64
	bundles := 2
	router := childQuotaRouter(t, &childQuotaProvisioning{
		updateSchoolFn: func(_ context.Context, id int64, changes organizationtenancy.SchoolChanges, operatorID int64, _ net.IP) (*organizationtenancy.School, error) {
			got, actor = changes, operatorID
			return &organizationtenancy.School{ID: id, ChildQuotaBundles: &bundles, ChildQuotaBundleSize: 50}, nil
		},
	})

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, childQuotaRequest(operatorRouteAccessToken(t, 42)))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, int64(42), actor)
	require.NotNil(t, got.ChildQuota)
	assert.Equal(t, &organizationtenancy.ChildQuota{Bundles: 2, BundleSize: 50}, got.ChildQuota.Quota)
	assert.Contains(t, rr.Body.String(), `"child_quota_bundles":2`)
	assert.Contains(t, rr.Body.String(), `"child_quota_bundle_size":50`)
}

// TestSchoolTokenCannotSetTheKinderkontingent pins that only operators set
// it: a school admin's token never reaches the owner.
func TestSchoolTokenCannotSetTheKinderkontingent(t *testing.T) {
	t.Parallel()
	router := childQuotaRouter(t, &childQuotaProvisioning{
		updateSchoolFn: func(context.Context, int64, organizationtenancy.SchoolChanges, int64, net.IP) (*organizationtenancy.School, error) {
			t.Fatal("a school token reached the Kinderkontingent")
			return nil, nil
		},
	})
	schoolToken, err := testutil.TestTokenAuth(t).CreateJWT(testutil.Claims{
		ID:    7,
		Sub:   "school-admin",
		Roles: []string{"admin"},
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, childQuotaRequest(schoolToken))

	assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rr.Code, rr.Body.String())
}
