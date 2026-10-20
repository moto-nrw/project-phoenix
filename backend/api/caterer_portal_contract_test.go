package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const catererContractPassword = "Kuechenpass-2026!"

type catererEnvelope struct {
	Code string          `json:"code"`
	Data json.RawMessage `json:"data"`
}

func decodeCatererResponse(t *testing.T, body []byte, into any) string {
	t.Helper()
	var envelope catererEnvelope
	require.NoError(t, json.Unmarshal(body, &envelope))
	if into != nil {
		require.NoError(t, json.Unmarshal(envelope.Data, into))
	}
	return envelope.Code
}

// catererInvitationLink reads the link the outbox mails to the caterer,
// as only the caterer can: no response to the school carries it.
func catererInvitationLink(t *testing.T, db *bun.DB, invitationID string) string {
	t.Helper()
	var raw string
	require.NoError(t, db.NewRaw(`SELECT payload->>'invitation_url' FROM platform.email_outbox
		WHERE kind = 'caterer_invitation' AND related_entity_id = ?::bigint`, invitationID).Scan(context.Background(), &raw))
	_, path, found := strings.Cut(raw, "/einladung/")
	require.True(t, found, raw)
	return path
}

// forgetContractCaterer removes what the acceptance wrote outside any
// school: the caterer, its sessions and its account.
func forgetContractCaterer(t *testing.T, db *bun.DB, email string) {
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.NewRaw(`DELETE FROM auth.caterers WHERE id IN (
			SELECT caterer_id FROM auth.caterer_accounts ca JOIN auth.accounts a ON a.id = ca.account_id WHERE a.email = ?)`, email).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM audit.auth_events WHERE account_id IN (SELECT id FROM auth.accounts WHERE email = ?)`, email).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM auth.tokens WHERE account_id IN (SELECT id FROM auth.accounts WHERE email = ?)`, email).Exec(ctx)
		_, _ = db.NewRaw(`DELETE FROM auth.accounts WHERE email = ?`, email).Exec(ctx)
	})
}

// checkCatererPortal drives the caterer portal (ADR 0075) through the
// production router: the school invites, the caterer accepts the mailed
// link, signs in to its own portal only, reads its school and loses it the
// moment the school ends the supply.
func checkCatererPortal(t *testing.T, graph *goldenRuntime) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, fmt.Sprintf("caterer-staff-%d@test.local", time.Now().UnixNano()))
	staffToken := testutil.MintTestJWT(t, testutil.TenantUserTestClaims(int(staff.ID), school, "meals:manage", "meals:read"))
	email := fmt.Sprintf("kueche-%d@example.com", time.Now().UnixNano())
	forgetContractCaterer(t, db, email)
	request := func(method, path, body, token string) (int, []byte) {
		response := checkpointRequest(graph, checkpointScenario{Method: method, Path: path, Body: body, Authenticated: token != ""}, token)
		return response.Code, response.Body.Bytes()
	}

	status, body := request(http.MethodPost, "/api/meal-caterer/invitations", fmt.Sprintf(`{"email":%q,"caterer_name":"Küche am Markt","delivery_address":"Schulstraße 1"}`, email), staffToken)
	require.Equal(t, http.StatusCreated, status, string(body))
	assert.NotContains(t, string(body), "einladung", "the school never sees the link")
	var invitation struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	decodeCatererResponse(t, body, &invitation)
	assert.Equal(t, "open", invitation.Status)
	link := catererInvitationLink(t, db, invitation.ID)

	status, body = request(http.MethodGet, "/caterer/invitations/"+link, "", "")
	require.Equal(t, http.StatusOK, status, string(body))
	var preview struct {
		Email             string `json:"email"`
		HasCatererAccount bool   `json:"has_caterer_account"`
	}
	decodeCatererResponse(t, body, &preview)
	assert.Equal(t, email, preview.Email)
	assert.False(t, preview.HasCatererAccount)

	schoolID, token, _ := strings.Cut(link, "/")
	status, body = request(http.MethodPost, fmt.Sprintf("/caterer/invitations/%d/%s/accept", school+1_000_000, token), `{"password":"x"}`, "")
	assert.Equal(t, http.StatusNotFound, status, "the link opens only the school it names: %s", body)
	status, body = request(http.MethodPost, "/caterer/invitations/"+schoolID+"/"+token+"/accept", fmt.Sprintf(`{"password":%q,"confirm_password":%q}`, catererContractPassword, catererContractPassword), "")
	require.Equal(t, http.StatusNoContent, status, string(body))

	status, body = request(http.MethodPost, "/caterer/auth/login", fmt.Sprintf(`{"email":%q,"password":"falsch"}`, email), "")
	require.Equal(t, http.StatusUnauthorized, status, string(body))
	status, body = request(http.MethodPost, "/caterer/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, catererContractPassword), "")
	require.Equal(t, http.StatusOK, status, string(body))
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	decodeCatererResponse(t, body, &tokens)
	catererToken := tokens.AccessToken

	status, body = request(http.MethodGet, "/caterer/schools", "", catererToken)
	require.Equal(t, http.StatusOK, status, string(body))
	var schools struct {
		CatererName string `json:"caterer_name"`
		Schools     []struct {
			ID string `json:"id"`
		} `json:"schools"`
	}
	decodeCatererResponse(t, body, &schools)
	assert.Equal(t, "Küche am Markt", schools.CatererName)
	require.Len(t, schools.Schools, 1)
	assert.Equal(t, schoolID, schools.Schools[0].ID)

	status, body = request(http.MethodPost, "/caterer/schools/"+schoolID+"/orders/999999999/confirm", "", catererToken)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "meals.order_not_found", decodeCatererResponse(t, body, nil))

	status, _ = request(http.MethodGet, "/caterer/schools", "", staffToken)
	assert.Equal(t, http.StatusUnauthorized, status, "a school session never opens the caterer portal")
	status, _ = request(http.MethodGet, "/api/meal-caterer/", "", catererToken)
	assert.Equal(t, http.StatusUnauthorized, status, "a caterer session never opens the school's API")
	status, _ = request(http.MethodGet, "/parent/me/children", "", catererToken)
	assert.Equal(t, http.StatusUnauthorized, status, "nor the parents portal")

	status, body = request(http.MethodGet, "/api/meal-caterer/", "", staffToken)
	require.Equal(t, http.StatusOK, status, string(body))
	var page struct {
		Supply *struct {
			CatererName string `json:"caterer_name"`
		} `json:"supply"`
	}
	decodeCatererResponse(t, body, &page)
	require.NotNil(t, page.Supply)
	assert.Equal(t, "Küche am Markt", page.Supply.CatererName)

	_, err := db.ExecContext(t.Context(), `
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'operations.meal_plan_enabled', 'true'::jsonb), (?, 'operations.meal_registration_enabled', 'true'::jsonb)
		ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, school, school)
	require.NoError(t, err)
	status, body = request(http.MethodGet, "/caterer/schools/"+schoolID+"/orders?from=2026-09-07&to=2026-09-11", "", catererToken)
	require.Equal(t, http.StatusOK, status, string(body))
	var orders struct {
		DeliveryAddress string `json:"delivery_address"`
		Days            []struct {
			Date string `json:"date"`
		} `json:"days"`
	}
	decodeCatererResponse(t, body, &orders)
	assert.Equal(t, "Schulstraße 1", orders.DeliveryAddress)
	assert.Empty(t, orders.Days, "a week before the supply started shows no day")
	for _, export := range []struct{ path, token, contentType string }{
		{"/caterer/orders/export?from=2026-09-07&to=2026-09-11&format=pdf", catererToken, "application/pdf"},
		{"/api/meal-caterer/orders/export?from=2026-09-07&to=2026-09-11&format=csv", staffToken, "text/csv; charset=utf-8"},
	} {
		response := checkpointRequest(graph, checkpointScenario{Method: http.MethodGet, Path: export.path, Authenticated: true}, export.token)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, export.contentType, response.Header().Get("Content-Type"))
	}

	status, body = request(http.MethodDelete, "/api/meal-caterer/supply", "", staffToken)
	require.Equal(t, http.StatusNoContent, status, string(body))
	status, body = request(http.MethodGet, "/caterer/schools/"+schoolID+"/orders?from=2026-09-07&to=2026-09-11", "", catererToken)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "meals.not_supplied", decodeCatererResponse(t, body, nil), "the end takes the school at once")
}

// checkCatererScopeMatrix walks the assembled router (ADR 0075): a caterer
// token unlocks no route of another portal, and no other portal's token
// unlocks a caterer route. A JWT-gated route answers 401 to the foreign
// token; a public one answers it exactly like an anonymous request.
func checkCatererScopeMatrix(t *testing.T, graph *goldenRuntime) {
	t.Parallel()
	tokenAuth, err := newSessionTokenAuth()
	require.NoError(t, err)
	mint := func(claims sessiontoken.AppClaims) string {
		token, mintErr := tokenAuth.CreateJWT(claims)
		require.NoError(t, mintErr)
		return token
	}
	caterer := mint(sessiontoken.AppClaims{ID: 42, Sub: "kueche@example.test", Roles: []string{"caterer"}, Scope: tenant.ScopeCaterer})
	others := map[string]string{
		"tenant": mint(sessiontoken.AppClaims{ID: 42, Sub: "admin@example.test", Roles: []string{"admin"}, Permissions: []string{"admin:*"}, IsAdmin: true, TenantID: testpkg.Tenant(t)}),
		"parent": mint(sessiontoken.AppClaims{ID: 42, Sub: "eltern@example.test", Roles: []string{"guardian"}, Scope: tenant.ScopeParent}),
		"school": mint(sessiontoken.AppClaims{ID: 42, Sub: "lehrkraft@example.test", Roles: []string{"lehrkraft"}, Scope: tenant.ScopeSchool, TenantID: testpkg.Tenant(t)}),
	}
	walkErr := chi.Walk(graph.router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		switch {
		case strings.HasPrefix(route, "/caterer/"):
			for portal, token := range others {
				assertForeignTokenUnlocksNothing(t, graph, method, route, token, portal)
			}
		case strings.HasPrefix(route, "/api/"), strings.HasPrefix(route, "/parent/"), strings.HasPrefix(route, "/school/"):
			assertForeignTokenUnlocksNothing(t, graph, method, route, caterer, "caterer")
		}
		return nil
	})
	require.NoError(t, walkErr)
}

func assertForeignTokenUnlocksNothing(t *testing.T, graph *goldenRuntime, method, pattern, token, portal string) {
	t.Helper()
	path := strings.ReplaceAll(chiParamPattern.ReplaceAllString(pattern, "1"), "*", "x")
	baseline := httptest.NewRecorder()
	graph.router.ServeHTTP(baseline, httptest.NewRequest(method, path, nil))
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	graph.router.ServeHTTP(response, request)
	if baseline.Code == http.StatusUnauthorized {
		assert.Equal(t, http.StatusUnauthorized, response.Code, "%s token on %s %s: %s", portal, method, pattern, strings.TrimSpace(response.Body.String()))
		return
	}
	assert.Equal(t, baseline.Code, response.Code, "%s token changed the public route %s %s", portal, method, pattern)
}
