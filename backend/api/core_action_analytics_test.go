package api

import (
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	projectJWT "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// checkCoreActionClassification is the route guard of the usage analytics:
// every writing route of the portal routers in the production router (and
// of the demo, mounted under APP_ENV=demo only) either names its core action
// or is explicitly not captured, and the table names no route that is gone.
func checkCoreActionClassification(t *testing.T, apiInstance *API) {
	t.Parallel()

	unclassified, stale, err := coreActionRouteGaps(apiInstance.Router)
	require.NoError(t, err)

	assert.Emptyf(t, unclassified,
		"these writing routes are not classified for the usage analytics. Add each to coreActions in backend/analytics/core_actions.go, with its event or as notCaptured (.claude/rules/usage-analytics.md):\n%s",
		strings.Join(unclassified, "\n"))
	assert.Emptyf(t, stale,
		"coreActions in backend/analytics/core_actions.go names routes the router no longer has; remove or rename them:\n%s",
		strings.Join(stale, "\n"))
}

// The guard itself: a router with a writing route the table does not know
// fails, and the table's routes that router lacks are reported as stale.
func TestCoreActionRouteGapsReportsUnclassifiedAndStaleRoutes(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	router.Post("/api/groups/", nil)
	router.Post("/api/brand-new-feature", nil)
	router.Get("/api/only-reads", nil)
	router.Post("/api/iot/checkin", nil)
	router.Post("/operator/schools", nil)

	unclassified, stale, err := coreActionRouteGaps(router)
	require.NoError(t, err)

	assert.Equal(t, []string{"POST /api/brand-new-feature"}, unclassified)
	assert.Contains(t, stale, "PUT /api/groups/{id}")
	assert.NotContains(t, stale, "POST /api/groups/")
	for _, route := range stale {
		assert.False(t, strings.HasPrefix(route, "POST /demo/"), "the demo routes come from the demo resource: %s", route)
	}
}

func TestAnalyticsActorFollowsTheTokenScope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		claims   projectJWT.AppClaims
		surface  string
		role     string
		schoolID int64
		ok       bool
	}{
		{
			name:    "OGS staff",
			claims:  projectJWT.AppClaims{TenantID: 12, Roles: []string{"Betreuung Nord"}},
			surface: "ogs", role: "staff", schoolID: 12, ok: true,
		},
		{
			name:    "OGS admin",
			claims:  projectJWT.AppClaims{TenantID: 12, IsAdmin: true, Scope: "tenant"},
			surface: "ogs", role: "admin", schoolID: 12, ok: true,
		},
		{
			name:    "staff preview is an admin at work",
			claims:  projectJWT.AppClaims{TenantID: 12, ReadOnly: true, ActingAdminID: 3},
			surface: "ogs", role: "admin", schoolID: 12, ok: true,
		},
		{
			name:    "parents have no school",
			claims:  projectJWT.AppClaims{Scope: "parent", TenantID: 12},
			surface: "parents", role: "guardian", ok: true,
		},
		{
			name:    "school portal",
			claims:  projectJWT.AppClaims{Scope: "school", TenantID: 12},
			surface: "school", role: "lehrkraft", schoolID: 12, ok: true,
		},
		{name: "operator is no portal", claims: projectJWT.AppClaims{Scope: "platform"}},
		{name: "tenant token without school", claims: projectJWT.AppClaims{}},
	}
	for _, tc := range cases {
		actor, ok := analyticsActor(tc.claims)
		assert.Equal(t, tc.ok, ok, tc.name)
		assert.Equal(t, tc.surface, actor.Surface, tc.name)
		assert.Equal(t, tc.role, actor.Role, tc.name)
		assert.Equal(t, tc.schoolID, actor.SchoolID, tc.name)
	}
}

// The account leaves the session only for the pseudonymous ID of an OGS
// account under Analyse-Freigabe (#3603): never from the parents or school
// portal, and never from the staff preview, where an admin looks at someone
// else's view.
func TestAnalyticsActorNamesTheAccountOnlyForOGS(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		claims    projectJWT.AppClaims
		accountID int64
	}{
		{"OGS staff", projectJWT.AppClaims{ID: 7, TenantID: 12}, 7},
		{"OGS admin", projectJWT.AppClaims{ID: 7, TenantID: 12, IsAdmin: true, Scope: "tenant"}, 7},
		{"staff preview", projectJWT.AppClaims{ID: 7, TenantID: 12, ReadOnly: true, ActingAdminID: 3}, 0},
		{"parents portal", projectJWT.AppClaims{ID: 7, Scope: "parent", TenantID: 12}, 0},
		{"school portal", projectJWT.AppClaims{ID: 7, Scope: "school", TenantID: 12}, 0},
	}
	for _, tc := range cases {
		actor, ok := analyticsActor(tc.claims)
		require.True(t, ok, tc.name)
		assert.Equal(t, tc.accountID, actor.AccountID, tc.name)
	}
}
