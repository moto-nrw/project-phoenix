package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

// Every portal's session routes refuse the writes of a read-only preview
// token before its scope gate runs; the parents portal once skipped it. The
// Authenticator (first link) needs a verified JWT, so the claims enter after
// it.
func TestEveryPortalSessionChainBlocksPreviewWrites(t *testing.T) {
	t.Parallel()

	// Every portal with session routes; the platform portal has none.
	for _, portal := range []tokenprimitives.PortalClaims{
		tokenprimitives.TenantClaims, tokenprimitives.SchoolClaims, tokenprimitives.ParentClaims,
		tokenprimitives.CatererClaims, tokenprimitives.OrgClaims,
	} {
		t.Run(portal.Name, func(t *testing.T) {
			t.Parallel()
			chain := SessionChain(portal)
			reached := false
			var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			})
			for i := len(chain) - 1; i >= 1; i-- {
				handler = chain[i](handler)
			}
			preview := sessiontoken.AppClaims{ID: 42, Scope: portal.Claim, TenantID: 12, ReadOnly: true, ActingAdminID: 7}
			req := httptest.NewRequest(http.MethodPost, "/api/anything", nil)
			req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxClaims, preview))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Contains(t, rec.Body.String(), readOnlyPreviewTestCode())
			assert.False(t, reached)
		})
	}
}
