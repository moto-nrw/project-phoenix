// Träger name on the public GET /auth/tenant/resolve endpoint (#4656). The
// school portal shows the Träger of its school, but only for a Träger with
// more than one operational school, and never data of the other schools.
package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type traegerSchool struct {
	id        int64
	name      string
	subdomain string
}

// newTraegerWithSchools creates one Träger with count active schools.
func newTraegerWithSchools(t *testing.T, db *bun.DB, count int) (*testpkg.Organization, []traegerSchool) {
	t.Helper()
	base := testpkg.UniqueTestTenantID(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		ID:     base,
		Name:   fmt.Sprintf("Träger %d", base),
		Slug:   fmt.Sprintf("traeger-%d", base),
		Active: true,
	})
	schools := make([]traegerSchool, 0, count)
	for range count {
		// Each school takes its own unique ID: the IDs are sequential, so an
		// offset from base would collide with a parallel test's tenant.
		id := testpkg.UniqueTestTenantID(t)
		school := traegerSchool{
			id:        id,
			name:      fmt.Sprintf("Schule %d", id),
			subdomain: fmt.Sprintf("traeger-schule-%d", id),
		}
		_, err := db.NewRaw(`INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
			VALUES (?, ?, ?, ?, ?, true)`,
			school.id, organization.ID, school.name, school.subdomain, school.subdomain).
			Exec(context.Background())
		require.NoError(t, err)
		schools = append(schools, school)
	}
	return organization, schools
}

func traegerResolveRouter(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()
	db, authRoute := setupAuthDependenciesRoute(t)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, authRoute.SchoolService, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)
	router := chi.NewRouter()
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
	return db, router
}

// resolveTraegerBody resolves the school and returns the raw body and the
// decoded data object.
func resolveTraegerBody(t *testing.T, router chi.Router, subdomain string) (string, map[string]any) {
	t.Helper()
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/auth/tenant/resolve?slug="+subdomain, nil))
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	return rr.Body.String(), resp.Data
}

func TestResolveTenant_TraegerWithSeveralSchoolsShowsItsNameOnly(t *testing.T) {
	t.Parallel()
	db, router := traegerResolveRouter(t)
	organization, schools := newTraegerWithSchools(t, db, 3)
	own, others := schools[0], schools[1:]

	body, data := resolveTraegerBody(t, router, own.subdomain)

	assert.Equal(t, organization.Name, data["traeger_name"])
	assert.Equal(t, own.name, data["name"])
	for _, other := range others {
		assert.NotContains(t, body, other.name, "the school portal must not see another school's name")
		assert.NotContains(t, body, other.subdomain, "the school portal must not see another school's address")
		assert.NotContains(t, body, strconv.FormatInt(other.id, 10), "the school portal must not see another school's ID")
	}
}

func TestResolveTenant_TraegerWithOneSchoolStaysUnchanged(t *testing.T) {
	t.Parallel()
	db, router := traegerResolveRouter(t)
	_, schools := newTraegerWithSchools(t, db, 1)

	_, data := resolveTraegerBody(t, router, schools[0].subdomain)

	assert.Contains(t, data, "name")
	assert.NotContains(t, data, "traeger_name", "a Träger with one OGS shows no Träger level")
}

func TestResolveTenant_InoperativeSchoolsDoNotCountForTheTraeger(t *testing.T) {
	t.Parallel()
	db, router := traegerResolveRouter(t)
	_, schools := newTraegerWithSchools(t, db, 3)
	_, err := db.NewRaw(`UPDATE platform.schools SET active = false WHERE id = ?`, schools[1].id).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, schools[2].id).Exec(context.Background())
	require.NoError(t, err)

	_, data := resolveTraegerBody(t, router, schools[0].subdomain)

	assert.Contains(t, data, "name")
	assert.NotContains(t, data, "traeger_name",
		"a Träger whose other schools are deactivated or deleted runs a single OGS")
}
