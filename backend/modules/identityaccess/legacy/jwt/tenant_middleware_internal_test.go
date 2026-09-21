package jwt

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// passThroughScope binds nothing; the bound tenant context is asserted with
// the real tenant runtime in api/common.
type passThroughScope struct{}

func (passThroughScope) BindTenant(ctx context.Context, _, _ int64, _ string) (context.Context, error) {
	return ctx, nil
}

func (passThroughScope) BindOrganization(ctx context.Context, _ int64, _ string) context.Context {
	return ctx
}

func (passThroughScope) BindScope(ctx context.Context, _ string) context.Context { return ctx }

func TestTenantMiddleware_PreservesConnectionState(t *testing.T) {
	t.Parallel()

	var gotTLSVersion uint16
	handler := TenantMiddleware(passThroughScope{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTLSVersion = r.TLS.Version
		w.WriteHeader(http.StatusOK)
	}))

	claims := AppClaims{ID: 42, Sub: "test@test.com", TenantID: 100, OrgID: 10, Scope: "tenant"}
	ctx := context.WithValue(context.Background(), CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, uint16(tls.VersionTLS13), gotTLSVersion)
}
