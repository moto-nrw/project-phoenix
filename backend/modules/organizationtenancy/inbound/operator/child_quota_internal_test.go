package operator

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvisioningResource_UpdateSchool_ChildQuota pins the wire contract of
// the Kinderkontingent on the existing school update (#3567): absent keeps
// it, null removes it, an object sets it with 50 as the default bundle size.
func TestProvisioningResource_UpdateSchool_ChildQuota(t *testing.T) {
	t.Parallel()
	const base = `"organization_id":7,"name":"School","slug":"school","subdomain":"school","active":true`
	for name, tc := range map[string]struct {
		field string
		want  *organizationtenancy.ChildQuotaChange
	}{
		"absent keeps it":            {field: ``, want: nil},
		"null removes it":            {field: `,"child_quota":null`, want: &organizationtenancy.ChildQuotaChange{}},
		"object with default size":   {field: `,"child_quota":{"bundles":2}`, want: &organizationtenancy.ChildQuotaChange{Quota: &organizationtenancy.ChildQuota{Bundles: 2, BundleSize: 50}}},
		"object with a special size": {field: `,"child_quota":{"bundles":3,"bundle_size":40}`, want: &organizationtenancy.ChildQuotaChange{Quota: &organizationtenancy.ChildQuota{Bundles: 3, BundleSize: 40}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var got *organizationtenancy.ChildQuotaChange
			called := false
			resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{
				updateSchoolFn: func(_ context.Context, _ int64, changes organizationtenancy.SchoolChanges, _ int64, _ net.IP) (*organizationtenancy.School, error) {
					called = true
					got = changes.ChildQuota
					return &organizationtenancy.School{ID: 10}, nil
				},
			}})
			req := httptest.NewRequest(http.MethodPut, "/operator/schools/10", bytes.NewBufferString(`{`+base+tc.field+`}`))
			req.Header.Set("Content-Type", "application/json")
			req = withOperatorClaims(req, 42)
			routeCtx := chi.NewRouteContext()
			routeCtx.URLParams.Add("id", "10")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
			rr := httptest.NewRecorder()

			resource.UpdateSchool(rr, req)

			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			require.True(t, called)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestProvisioningResource_UpdateSchool_ChildQuotaMalformed(t *testing.T) {
	t.Parallel()
	resource := NewProvisioningResource(ProvisioningConfig{Service: &mockProvisioningService{}})
	req := httptest.NewRequest(http.MethodPut, "/operator/schools/10",
		bytes.NewBufferString(`{"organization_id":7,"name":"S","slug":"s","subdomain":"s","child_quota":{"bundles":"zwei"}}`))
	req.Header.Set("Content-Type", "application/json")
	req = withOperatorClaims(req, 42)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "10")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rr := httptest.NewRecorder()

	resource.UpdateSchool(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}
