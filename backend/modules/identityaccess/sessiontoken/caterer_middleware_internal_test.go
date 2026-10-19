package sessiontoken

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func serveWithClaims(middleware func(http.Handler) http.Handler, claims AppClaims) int {
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ctx := context.WithValue(context.Background(), CtxClaims, claims)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	return rec.Code
}

// ADR 0074: a caterer token works on the caterer portal only, and no other
// token works there.
func TestCatererScopeStaysInTheCatererPortal(t *testing.T) {
	t.Parallel()
	caterer := AppClaims{ID: 42, Scope: claimScopeCaterer}
	assert.True(t, caterer.IsCatererScope())
	assert.Equal(t, http.StatusOK, serveWithClaims(CatererMiddleware(passThroughScope{}), caterer))
	assert.Equal(t, http.StatusUnauthorized, serveWithClaims(CatererMiddleware(passThroughScope{}), AppClaims{Scope: claimScopeCaterer}), "no account")

	for _, other := range []AppClaims{
		{ID: 42, TenantID: 100, Scope: "tenant"},
		{ID: 42, TenantID: 100, Scope: claimScopeSchool},
		{ID: 42, Scope: claimScopeParent},
		{ID: 42, Scope: claimScopePlatform},
		{ID: 42, TenantID: 100, Scope: ""},
	} {
		assert.Equal(t, http.StatusUnauthorized, serveWithClaims(CatererMiddleware(passThroughScope{}), other), other.Scope)
		assert.False(t, other.IsCatererScope())
	}

	withSchool := caterer
	withSchool.TenantID = 100
	assert.Equal(t, http.StatusUnauthorized, serveWithClaims(TenantMiddleware(passThroughScope{}), withSchool), "tenant routes refuse a caterer, even with a school")
	assert.Equal(t, http.StatusUnauthorized, serveWithClaims(ParentMiddleware(passThroughScope{}), caterer))
	assert.Equal(t, http.StatusUnauthorized, serveWithClaims(SchoolMiddleware(passThroughScope{}), withSchool))
}
