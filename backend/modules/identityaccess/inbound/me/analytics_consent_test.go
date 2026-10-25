package me

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const consentAccountID = 4711

// consentDouble records the decisions of the resource; findErr and
// decideErr inject the capability's failures.
type consentDouble struct {
	answers   map[int64]bool
	findErr   error
	decideErr error
}

func (d *consentDouble) FindAnalyticsConsent(_ context.Context, accountID int64) (identityaccess.AnalyticsConsent, error) {
	if d.findErr != nil {
		return identityaccess.AnalyticsConsent{}, d.findErr
	}
	answer, ok := d.answers[accountID]
	if !ok {
		return identityaccess.AnalyticsConsent{}, nil
	}
	return identityaccess.AnalyticsConsent{Granted: &answer}, nil
}

func (d *consentDouble) DecideAnalyticsConsent(ctx context.Context, accountID int64, granted bool) (identityaccess.AnalyticsConsent, error) {
	if d.decideErr != nil {
		return identityaccess.AnalyticsConsent{}, d.decideErr
	}
	d.answers[accountID] = granted
	return d.FindAnalyticsConsent(ctx, accountID)
}

// consentRouter stands in for the root chain: it places verified claims on
// the request like sessiontoken.Authenticator does.
func consentRouter(consent identityaccess.AccountAnalyticsConsent, claims sessiontoken.AppClaims) http.Handler {
	return NewAnalyticsConsentResource(consent).Router(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessiontoken.CtxClaims, claims)))
			})
		})
	})
}

func serveConsent(t *testing.T, handler http.Handler, method, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	return rec, decoded
}

func TestAnalyticsConsentStartsUndecidedAndRecordsTheOwnAnswer(t *testing.T) {
	t.Parallel()

	consent := &consentDouble{answers: map[int64]bool{}}
	for _, scope := range []string{"", "parent", "school"} {
		router := consentRouter(consent, sessiontoken.AppClaims{ID: consentAccountID, Scope: scope, TenantID: 42})

		rec, body := serveConsent(t, router, http.MethodGet, "")
		require.Equal(t, http.StatusOK, rec.Code, "scope %q", scope)
		assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		data := body["data"].(map[string]any)
		require.Contains(t, data, "granted")
		assert.Nil(t, data["granted"], "undecided reads as null, never as false")

		rec, body = serveConsent(t, router, http.MethodPut, `{"granted":true}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, true, body["data"].(map[string]any)["granted"])
		assert.True(t, consent.answers[consentAccountID])

		rec, body = serveConsent(t, router, http.MethodPut, `{"granted":false}`)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, false, body["data"].(map[string]any)["granted"])
		_, body = serveConsent(t, router, http.MethodGet, "")
		assert.Equal(t, false, body["data"].(map[string]any)["granted"])
		delete(consent.answers, consentAccountID)
	}
}

// An operator and the Träger portal (#4649) have no portal analytics, and a
// staff-view preview carries the target's identity: none reads or decides for
// that account.
func TestAnalyticsConsentRefusesOperatorAndPreviewSessions(t *testing.T) {
	t.Parallel()

	for name, claims := range map[string]sessiontoken.AppClaims{
		"operator":   {ID: consentAccountID, Scope: "platform"},
		"träger":     {ID: consentAccountID, Scope: "org", OrgID: 1},
		"preview":    {ID: consentAccountID, TenantID: 42, ReadOnly: true},
		"no account": {TenantID: 42},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			consent := &consentDouble{answers: map[int64]bool{}}
			router := consentRouter(consent, claims)
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				rec, body := serveConsent(t, router, method, `{"granted":true}`)
				assert.Equal(t, http.StatusForbidden, rec.Code, method)
				assert.NotEmpty(t, body["code"])
			}
			assert.Empty(t, consent.answers)
		})
	}
}

func TestAnalyticsConsentNeedsAnAnswer(t *testing.T) {
	t.Parallel()

	consent := &consentDouble{answers: map[int64]bool{}}
	router := consentRouter(consent, sessiontoken.AppClaims{ID: consentAccountID, TenantID: 42})
	for _, body := range []string{`{}`, `{"granted":null}`, `not json`} {
		rec, decoded := serveConsent(t, router, http.MethodPut, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.NotEmpty(t, decoded["code"])
	}
	assert.Empty(t, consent.answers)
}

func TestAnalyticsConsentMapsCapabilityFailures(t *testing.T) {
	t.Parallel()

	claims := sessiontoken.AppClaims{ID: consentAccountID, TenantID: 42}
	missing := consentRouter(&consentDouble{findErr: identityaccess.ErrAccountNotFound, decideErr: identityaccess.ErrAccountNotFound}, claims)
	broken := consentRouter(&consentDouble{findErr: errors.New("db down"), decideErr: errors.New("db down")}, claims)
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		rec, _ := serveConsent(t, missing, method, `{"granted":true}`)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, method)
		rec, _ = serveConsent(t, broken, method, `{"granted":true}`)
		assert.Equal(t, http.StatusInternalServerError, rec.Code, method)
	}
}
