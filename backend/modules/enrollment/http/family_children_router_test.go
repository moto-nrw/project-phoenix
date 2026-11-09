package enrollmenthttp_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The children of the family on the status page of a registration (#6335).
// The status token is the only key: it reads and writes the number of its
// own registration, the school asks only when it booked the contracts and
// the parents' contributions, and the number closes once a child is
// accepted, when the contract carries it.

// familyChildrenSettings books the contracts and the parents'
// contributions unless asks is false.
type familyChildrenSettings struct {
	stubTakeoverSettings
	asks bool
}

func (s familyChildrenSettings) HasTenantOverride(ctx context.Context, key string) (bool, error) {
	if strings.HasPrefix(key, "module.") {
		return true, nil
	}
	return s.stubTakeoverSettings.HasTenantOverride(ctx, key)
}

func (s familyChildrenSettings) ResolveBool(ctx context.Context, key string) (bool, error) {
	if key == "module.vertraege" || key == "module.elternbeitraege" {
		return s.asks, nil
	}
	return s.stubTakeoverSettings.ResolveBool(ctx, key)
}

type familyChildrenEnv struct {
	*takeoverLockEnv
	other string
}

func setupFamilyChildren(t *testing.T, asks bool) familyChildrenEnv {
	t.Helper()
	env := setupTakeoverLockTest(t)
	settings := familyChildrenSettings{asks: asks}
	repos, err := api.NewEnrollmentTestRepositories(testpkg.NewDatabase(t, env.db), owners.NewTestAuditStore(testpkg.NewDatabase(t, env.db)))
	require.NoError(t, err)
	reposCare := api.NewEnrollmentCarePlanRows(testpkg.NewDatabase(t, env.db))
	reposSchools := api.NewOrganizationTestRows(testpkg.NewDatabase(t, env.db))
	intake := owners.NewEnrollmentIntake(owners.EnrollmentIntakeSources{
		Requests: repos.Enrollment(), Children: repos.Enrollment(), Guardians: repos.Enrollment(),
		CareOfferingRepo: enrollmentCompose.NewCareOfferingRecords(reposCare.CarePlan),
		Capacity:         owners.NewEnrollmentOfferingCapacity(enrollmentCompose.NewCareOfferingRecords(reposCare.CarePlan), repos.Enrollment(), settings),
		Catalog:          repos.Enrollment(), SchoolRepo: capabilitySchools{schools: reposSchools.School},
		Notifications: NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: settings}, discardingOutbox{}, capabilitySchools{schools: reposSchools.School}),
		RateLimitRepo: repos.Enrollment(), OutboxEnqueuer: discardingOutbox{}, Settings: settings,
		FrontendURL: "http://localhost:3000", ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	})
	resource := enrollmentAPI.NewResource(nil, nil, newFixtureRequestRoot(intake), nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nativeUploads.UploadFiles{}, enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation})
	resource.FamilyChildren = intake
	env.router = testpkg.TenantRuntimeMiddleware(t, env.db)(resource.Router(api.TenantSessionChain))

	// A second registration of another family at the same school.
	other, err := newFixtureRequestRoot(intake).Submit(testpkg.TenantContext(env.tenantID), enrollmentAPI.SubmitRequest{
		TenantID: env.tenantID, PhaseID: env.phaseID, GuardianFirstName: "Olga", GuardianLastName: "Andere",
		GuardianEmail: "olga.andere@example.com",
		ConsentFlags:  testFormValues(map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": false}),
		Children: []enrollmentAPI.SubmitChild{{FirstName: "Ole", LastName: "Andere", DateOfBirth: calendar.NewDate(2018, 5, 1),
			TargetGradeLevel: testpkg.Int16Ptr(1)}},
	})
	require.NoError(t, err)
	return familyChildrenEnv{takeoverLockEnv: env, other: other.Request.StatusToken}
}

type familyChildrenEnvelope struct {
	Data struct {
		Children *int `json:"children"`
		Asked    bool `json:"asked"`
		CanState bool `json:"can_state"`
	} `json:"data"`
}

func (env familyChildrenEnv) put(t *testing.T, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/requests/"+token+"/family-children", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func decodeFamilyChildren(t *testing.T, rec *httptest.ResponseRecorder) familyChildrenEnvelope {
	t.Helper()
	var out familyChildrenEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	return out
}

func (env familyChildrenEnv) stored(t *testing.T, token string) *int {
	t.Helper()
	var children *int
	require.NoError(t, env.db.NewRaw(`SELECT family_children FROM enrollment.requests WHERE status_token = ?`, token).
		Scan(context.Background(), &children))
	return children
}

func TestFamilyChildrenOnTheStatusPageStayInTheOwnRegistration(t *testing.T) {
	t.Parallel()
	env := setupFamilyChildren(t, true)

	rec := env.get(t, "/requests/"+env.token+"/family-children")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	read := decodeFamilyChildren(t, rec)
	assert.True(t, read.Data.Asked)
	assert.True(t, read.Data.CanState)
	assert.Nil(t, read.Data.Children)

	rec = env.put(t, env.token, `{"children":3}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, decodeFamilyChildren(t, rec).Data.Children)
	assert.Equal(t, 3, *decodeFamilyChildren(t, rec).Data.Children)
	rec = env.put(t, env.token, `{"children":2}`)
	require.Equal(t, http.StatusOK, rec.Code, "the parents correct their number: %s", rec.Body.String())
	require.NotNil(t, env.stored(t, env.token))
	assert.Equal(t, 2, *env.stored(t, env.token))
	assert.Nil(t, env.stored(t, env.other), "the token writes only its own registration")

	for _, body := range []string{`{"children":0}`, `{"children":21}`} {
		rec = env.put(t, env.token, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "enrollment.family_children_invalid")
	}
	for _, token := range []string{"unknown-token", env.token + "x"} {
		rec = env.put(t, token, `{"children":4}`)
		assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "enrollment.status_link_invalid")
	}

	// An expired status link opens nothing.
	_, err := env.db.ExecContext(context.Background(), `UPDATE enrollment.requests SET status_token_expires = NOW() - INTERVAL '1 day'
		WHERE status_token = ?`, env.other)
	require.NoError(t, err)
	rec = env.put(t, env.other, `{"children":4}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Nil(t, env.stored(t, env.other))

	// Once a child is accepted the contract carries the number.
	env.takeOver(t, env.request.Children[0].ID, "Lina")
	rec = env.get(t, "/requests/"+env.token+"/family-children")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, decodeFamilyChildren(t, rec).Data.CanState)
	rec = env.put(t, env.token, `{"children":5}`)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "enrollment.family_children_closed")
	assert.Equal(t, 2, *env.stored(t, env.token))
}

func TestFamilyChildrenAreAskedOnlyWithContractsAndContributions(t *testing.T) {
	t.Parallel()
	env := setupFamilyChildren(t, false)
	rec := env.get(t, "/requests/"+env.token+"/family-children")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	read := decodeFamilyChildren(t, rec)
	assert.False(t, read.Data.Asked)
	assert.False(t, read.Data.CanState)
	rec = env.put(t, env.token, `{"children":3}`)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Nil(t, env.stored(t, env.token), "a school that does not use the number stores none")
}

// The update names the school below row level security: a request of
// another school with the same ID stays untouched.
func TestSetRequestFamilyChildrenKeepsTheSchoolBelowRLS(t *testing.T) {
	t.Parallel()
	env := setupFamilyChildren(t, true)
	other := setupFamilyChildren(t, true)
	var otherID int64
	require.NoError(t, env.db.NewRaw(`SELECT id FROM enrollment.requests WHERE status_token = ?`, other.token).Scan(context.Background(), &otherID))
	store := enrollmentCompose.NewRequestFamilyChildren()
	ctx := testpkg.WithTestTenantRuntime(t, testpkg.TenantContext(env.tenantID))
	err := testpkg.WithinTenantTransaction(ctx, env.db, env.tenantID, func(ctx context.Context, _ bun.Tx) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, ok := testpkg.TransactionFromContext(ctx)
		require.True(t, ok)
		var bypasses bool
		require.NoError(t, tx.(bun.Tx).NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		return store.SetRequestFamilyChildren(ctx, otherID, 4)
	})
	require.ErrorIs(t, err, capability.ErrRequestNotFound)
	assert.Nil(t, other.stored(t, other.token), "the other school's request keeps its number")
}
