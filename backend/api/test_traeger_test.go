// The test system of a new Träger (#4658, spec #4611 user story 125): the
// moto team creates a test Träger with hidden test schools, fills them with
// invented children, and the Träger-Portal shows that it is a test system.
// Sample children never enter a real Träger.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	tenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/erasureledger"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testTraegerFixture holds the operator routes, sample children included,
// over an isolated database.
type testTraegerFixture struct {
	db         *bun.DB
	ctx        context.Context
	operatorID int64
	operator   http.Handler
	identity   identityaccess.Owner
}

func newTestTraegerFixture(t *testing.T) testTraegerFixture {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	service, identity := buildOperatorProvisioningWithIdentity(t, db)
	imports, err := NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service}),
		newSampleChildrenRoute(service, imports.Import))
	return testTraegerFixture{
		db: db, ctx: provisioningContext(t, db), operatorID: testpkg.CreateTestOperator(t, db).ID,
		operator: operator, identity: identity,
	}
}

func (f testTraegerFixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(f.ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(f.operatorID), Scope: "platform"}))
	rec := httptest.NewRecorder()
	f.operator.ServeHTTP(rec, req)
	return rec
}

type createdTestTraeger struct {
	Organization struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		TestSystem bool   `json:"test_system"`
	} `json:"organization"`
	Schools []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Subdomain string `json:"subdomain"`
		Hidden    bool   `json:"hidden"`
		Active    bool   `json:"active"`
	} `json:"schools"`
}

func (f testTraegerFixture) createTestTraeger(t *testing.T, slug string, schools int) createdTestTraeger {
	t.Helper()
	rec := f.call(t, http.MethodPost, "/organizations/test-traeger", map[string]any{
		"name": "Test-Träger " + slug, "slug": slug, "school_count": schools, "contact_email": "buero@traeger.example",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[createdTestTraeger](t, rec.Body.Bytes())
}

type sampleChildrenAnswer struct {
	CreatedCount int `json:"created_count"`
	Schools      []struct {
		SchoolID     int64 `json:"school_id"`
		CreatedCount int   `json:"created_count"`
		SkippedCount int   `json:"skipped_count"`
	} `json:"schools"`
}

func (f testTraegerFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(query, args...).Scan(f.ctx, &count))
	return count
}

func TestOperatorCreatesATestTraegerWithHiddenTestSchoolsAndSampleChildren(t *testing.T) {
	t.Parallel()
	f := newTestTraegerFixture(t)
	slug := fmt.Sprintf("test-%d", testpkg.UniqueTestTenantID(t))

	created := f.createTestTraeger(t, slug, 2)
	assert.True(t, created.Organization.TestSystem, "the Träger is marked as a test Träger")
	require.Len(t, created.Schools, 2)
	for number, school := range created.Schools {
		assert.Equal(t, fmt.Sprintf("Test-Schule %d", number+1), school.Name)
		assert.Equal(t, fmt.Sprintf("%s-test-%d", slug, number+1), school.Subdomain)
		assert.True(t, school.Hidden, "a test school stays off the public school list")
		assert.True(t, school.Active)
	}
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM platform.organizations WHERE id = ? AND test_system`, created.Organization.ID))
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'create' AND resource_type = 'organization' AND resource_id = ?
		AND changes->>'test_system' = 'true'`, f.operatorID, created.Organization.ID), "the creation is in the operator audit")
	assert.Equal(t, 2, f.count(t, `SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'create' AND resource_type = 'school' AND resource_id IN (?, ?)`,
		f.operatorID, created.Schools[0].ID, created.Schools[1].ID), "every test school is in the operator audit")

	path := fmt.Sprintf("/organizations/%d/sample-children", created.Organization.ID)
	rec := f.call(t, http.MethodPost, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	filled := decodeSchoolPortalData[sampleChildrenAnswer](t, rec.Body.Bytes())
	assert.Equal(t, 48, filled.CreatedCount, "each test school gets 24 invented children")
	for _, school := range created.Schools {
		assert.Equal(t, 24, f.count(t, `SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ? AND deleted_at IS NULL`, school.ID))
		assert.Equal(t, 4, f.count(t, `SELECT count(DISTINCT school_class) FROM users.student_school_memberships WHERE tenant_id = ? AND deleted_at IS NULL`, school.ID))
	}
	assert.Zero(t, f.count(t, `SELECT count(*) FROM users.student_guardian_relationships WHERE tenant_id IN (?, ?)`,
		created.Schools[0].ID, created.Schools[1].ID), "sample children have no guardians")
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'create' AND resource_type = 'sample_children' AND resource_id = ?`,
		f.operatorID, created.Organization.ID), "the request is in the operator audit")

	rec = f.call(t, http.MethodPost, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := decodeSchoolPortalData[sampleChildrenAnswer](t, rec.Body.Bytes())
	assert.Zero(t, again.CreatedCount, "a second run creates nothing twice")
	for _, school := range again.Schools {
		assert.Equal(t, 24, school.SkippedCount)
	}
	assert.Equal(t, 24, f.count(t, `SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ? AND deleted_at IS NULL`, created.Schools[0].ID))
}

func TestOperatorTestTraegerRefusesInvalidOrders(t *testing.T) {
	t.Parallel()
	f := newTestTraegerFixture(t)
	slug := fmt.Sprintf("test-%d", testpkg.UniqueTestTenantID(t))
	order := func(changes map[string]any) map[string]any {
		body := map[string]any{"name": "Test-Träger", "slug": slug, "school_count": 3, "contact_email": "buero@traeger.example"}
		for key, value := range changes {
			body[key] = value
		}
		return body
	}
	for name, body := range map[string]map[string]any{
		"too many schools":     order(map[string]any{"school_count": 6}),
		"no contact address":   order(map[string]any{"contact_email": ""}),
		"slug too long":        order(map[string]any{"slug": slug + "-mit-einem-sehr-langen-namen-fuer-die-subdomain-der-schulen"}),
		"no name":              order(map[string]any{"name": " "}),
		"slug with uppercase!": order(map[string]any{"slug": "Test Träger"}),
	} {
		rec := f.call(t, http.MethodPost, "/organizations/test-traeger", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
	}
	assert.Zero(t, f.count(t, `SELECT count(*) FROM platform.organizations WHERE slug LIKE ?`, slug+"%"), "a refused order creates nothing")

	f.createTestTraeger(t, slug, 1)
	rec := f.call(t, http.MethodPost, "/organizations/test-traeger", order(nil))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, 1, f.count(t, `SELECT count(*) FROM platform.schools AS s JOIN platform.organizations AS o ON o.id = s.organization_id WHERE o.slug = ?`, slug),
		"the refused second order adds no school")
}

func TestSampleChildrenNeverEnterARealTraeger(t *testing.T) {
	t.Parallel()
	f := newTestTraegerFixture(t)
	service := buildOperatorProvisioning(t, f.db)
	realTraeger, realSchool := provisionTestSchool(t, f.db, service, f.operatorID, "echt")

	rec := f.call(t, http.MethodPost, fmt.Sprintf("/organizations/%d/sample-children", realTraeger), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"import.not_test_traeger"`)
	assert.Zero(t, f.count(t, `SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ? AND deleted_at IS NULL`, realSchool))

	rec = f.call(t, http.MethodPost, "/organizations/987654321/sample-children", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	created := f.createTestTraeger(t, fmt.Sprintf("test-%d", testpkg.UniqueTestTenantID(t)), 1)
	_, err := f.db.ExecContext(f.ctx, `UPDATE platform.organizations SET active = false WHERE id = ?`, created.Organization.ID)
	require.NoError(t, err)
	rec = f.call(t, http.MethodPost, fmt.Sprintf("/organizations/%d/sample-children", created.Organization.ID), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"import.test_traeger_inactive"`)
	assert.Zero(t, f.count(t, `SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ? AND deleted_at IS NULL`, created.Schools[0].ID))
	assert.Zero(t, f.count(t, `SELECT count(*) FROM platform.operator_audit_log WHERE resource_type = 'sample_children'`),
		"a refused request writes no audit entry")
}

type traegerFrame struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	TestSystem bool   `json:"test_system"`
}

func traegerOrganizationFrame(t *testing.T, router http.Handler, token string) (*httptest.ResponseRecorder, traegerFrame) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/traeger/organization/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec, traegerFrame{}
	}
	return rec, decodeSchoolPortalData[traegerFrame](t, rec.Body.Bytes())
}

func TestTraegerPortalMarksATestTraegerAndOnlyNamesTheSessionsTraeger(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerPortal(t)
	password := testpkg.NewTestPassword()
	testTraeger := traegerOrganization(t, db, "Test-Träger Portal", true)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.organizations SET test_system = true WHERE id = ?`, testTraeger)
	require.NoError(t, err)
	realTraeger := traegerOrganization(t, db, "Echter Träger Portal", true)
	tester := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, testTraeger, tester.ID, identityaccess.TraegerRoleReadOnly)
	office := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, realTraeger, office.ID, identityaccess.TraegerRoleAdministration)

	code, answer := traegerLogin(t, router, tester.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	rec, frame := traegerOrganizationFrame(t, router, answer.AccessToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, traegerFrame{ID: testTraeger, Name: "Test-Träger Portal", TestSystem: true}, frame)

	code, answer = traegerLogin(t, router, office.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	rec, frame = traegerOrganizationFrame(t, router, answer.AccessToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, traegerFrame{ID: realTraeger, Name: "Echter Träger Portal"}, frame, "a real Träger is not marked and never sees another Träger")

	noRight := testpkg.MintTestJWT(t, sessiontoken.AppClaims{ID: int(office.ID), Sub: office.Email, Scope: "org", OrgID: realTraeger, Roles: []string{identityaccess.TraegerRoleBilling}})
	rec, _ = traegerOrganizationFrame(t, router, noRight)
	assert.Equal(t, http.StatusForbidden, rec.Code, "a role without the right gets 403")

	_, err = db.ExecContext(context.Background(), `UPDATE platform.organizations SET active = false WHERE id = ?`, realTraeger)
	require.NoError(t, err)
	rec, _ = traegerOrganizationFrame(t, router, answer.AccessToken)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a deactivated Träger ends the session")
}

// schoolErasureRows counts the rows of a school in every table the school
// erasure plan names.
func schoolErasureRows(t *testing.T, db *bun.DB, schoolID int64) int64 {
	t.Helper()
	var raw string
	require.NoError(t, db.NewRaw(`SELECT platform.school_erasure_plan()::text`).Scan(t.Context(), &raw))
	var plan []struct{ Name, Predicate string }
	require.NoError(t, json.Unmarshal([]byte(raw), &plan))
	var total int64
	for _, entry := range plan {
		var n int64
		require.NoError(t, db.NewRaw("SELECT count(*) FROM "+entry.Name+" AS r WHERE "+strings.ReplaceAll(entry.Predicate, "$1", "?"), schoolID).Scan(t.Context(), &n))
		total += n
	}
	return total
}

func TestATestTraegerWithSampleChildrenIsErasedCompletely(t *testing.T) {
	t.Parallel()
	f := newTestTraegerFixture(t)
	created := f.createTestTraeger(t, fmt.Sprintf("test-%d", testpkg.UniqueTestTenantID(t)), 2)
	organizationID := created.Organization.ID
	rec := f.call(t, http.MethodPost, fmt.Sprintf("/organizations/%d/sample-children", organizationID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, school := range created.Schools {
		require.Positive(t, schoolErasureRows(t, f.db, school.ID), "the sample children are in the test school")
	}

	// The test Träger leaves like every Träger (#4657): deactivated, with a
	// due erasure order, through the erasure CLI's service.
	_, err := f.db.ExecContext(f.ctx, `UPDATE platform.organizations SET active = false WHERE id = ?`, organizationID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(f.ctx, `INSERT INTO platform.organization_erasure_orders (organization_id, order_reference, confirmed_on, erase_not_before, recorded_by)
		VALUES (?, 'TEST-TRAEGER', '2026-01-02', '2026-01-03', ?)`, organizationID, f.operatorID)
	require.NoError(t, err)
	erasure, err := tenancyCompose.NewErasure(f.db, objects.NewLocal(t.TempDir()), erasureledger.New(testErasureLedger(t), securityruntime.Fingerprint))
	require.NoError(t, err)
	_, err = erasure.Run(t.Context(), organizationtenancy.ErasureRequest{
		OrganizationID: organizationID, OperatorID: f.operatorID, OrderReference: "TEST-TRAEGER",
		Confirmation: fmt.Sprintf("organization:%d", organizationID), PrerequisitesChecked: true, WritersStopped: true,
		LockTimeout: time.Second, StatementTimeout: time.Minute,
	})
	require.NoError(t, err)

	for _, school := range created.Schools {
		assert.Zero(t, schoolErasureRows(t, f.db, school.ID), "no row of test school %d is left", school.ID)
	}
	assert.Zero(t, f.count(t, `SELECT count(*) FROM platform.schools WHERE organization_id = ?`, organizationID))
	assert.Zero(t, f.count(t, `SELECT count(*) FROM platform.organizations WHERE id = ?`, organizationID), "the test Träger itself is gone")
}

func testErasureLedger(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger")
	require.NoError(t, erasureledger.Initialize(path))
	return path
}
