//go:build testhooks

package worktimemodels_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type catalogRow struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	ArchivedAt         *string `json:"archived_at"`
	AssignedStaffCount *int    `json:"assigned_staff_count"`
}

func catalogRequest(t *testing.T, router chi.Router, method, path string) (*http.Response, json.RawMessage) {
	t.Helper()
	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{"time_tracking:manage"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	request := testutil.NewAuthenticatedRequest(t, method, path, nil, testutil.WithJWTBearer(token))
	recorder := testutil.ExecuteRequestForTest(t, router, request)
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if recorder.Code < 300 {
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope), recorder.Body.String())
	}
	return recorder.Result(), envelope.Data
}

func bindToModel(t *testing.T, db *bun.DB, staffID, modelID int64) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE users.staff_employment_profiles SET work_time_model_id = ? WHERE membership_id = ?`, modelID, staffID).
		Exec(context.Background())
	require.NoError(t, err)
}

func listCatalog(t *testing.T, router chi.Router) map[string]catalogRow {
	t.Helper()
	response, data := catalogRequest(t, router, http.MethodGet, "/?with_usage=true")
	require.Equal(t, http.StatusOK, response.StatusCode)
	var rows []catalogRow
	require.NoError(t, json.Unmarshal(data, &rows))
	byName := make(map[string]catalogRow, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	return byName
}

// with_usage=true reports the live assignees per template (#6891) in a
// statement count that does not grow with templates or assignees.
func TestWorkTimeModelsListWithUsageCountsLiveAssigneesFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	models := api.NewAbsenceTypeTestCapability(testpkg.NewDatabase(t, db))
	router := testpkg.MountedQueryBudgetRouter("/api/work-time-models", api.NewWorkTimeModelsRouterForBudgetTest(models, func(context.Context) {}))
	ctx := testpkg.Ctx(t)

	create := func(name string) workforce.WorkTimeModel {
		model, err := models.CreateWorkTimeModel(ctx, workforce.CreateWorkTimeModel{WorkTimeModelFields: budgetWorkTimeModelFields(name, 1)})
		require.NoError(t, err)
		testpkg.OwnTestWorkTimeModel(t, db, model.ID)
		return model
	}
	fullTime := create("Vollzeit")
	create("Ungenutzt")
	foreign := testpkg.NewTenantScope(t, db)
	foreignModel, err := models.CreateWorkTimeModel(foreign.Context(), workforce.CreateWorkTimeModel{WorkTimeModelFields: budgetWorkTimeModelFields("Foreign", 1)})
	require.NoError(t, err)
	testpkg.OwnTestWorkTimeModel(t, db, foreignModel.ID)

	retired := testpkg.CreateTestStaff(t, db, "Catalog", "Retired")
	bindToModel(t, db, retired.ID, fullTime.ID)
	_, err = db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = now() WHERE id = ?`, retired.ID).Exec(context.Background())
	require.NoError(t, err)

	counter := testpkg.CaptureQueries(t, db)
	var previous int
	for _, assignees := range []int{1, 4} {
		for index := 0; index < assignees-previous; index++ {
			staff := testpkg.CreateTestStaff(t, db, "Catalog", fmt.Sprintf("Live %d-%d", assignees, index))
			bindToModel(t, db, staff.ID, fullTime.ID)
		}
		counter.Reset()
		catalog := listCatalog(t, router)
		queries := counter.Measurement(counter.Queries())

		require.Len(t, catalog, 2, "foreign templates stay invisible")
		require.NotNil(t, catalog["Vollzeit"].AssignedStaffCount)
		assert.Equal(t, assignees, *catalog["Vollzeit"].AssignedStaffCount, "only live assignees count")
		require.NotNil(t, catalog["Ungenutzt"].AssignedStaffCount, "an unused template reports zero, not nothing")
		assert.Zero(t, *catalog["Ungenutzt"].AssignedStaffCount)
		require.Zero(t, counter.WriteRows(), "the list writes nothing")
		querybudgets.AssertQueryBudget(t, "api.workforce.work_time_models.list.with_usage", queries)
		previous = assignees
	}

	response, data := catalogRequest(t, router, http.MethodGet, "/")
	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.NotContains(t, string(data), "assigned_staff_count", "the plain list stays without the usage read")
}

// Archiving takes a template out of the choice and restoring offers it again;
// both answer with the template and stay inside the caller's school.
func TestWorkTimeModelsArchiveAndRestore(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	models := api.NewAbsenceTypeTestCapability(testpkg.NewDatabase(t, db))
	notified := 0
	router := chi.NewRouter()
	router.Mount("/", api.NewWorkTimeModelsRouterForBudgetTest(models, func(context.Context) { notified++ }))
	model, err := models.CreateWorkTimeModel(testpkg.Ctx(t), workforce.CreateWorkTimeModel{WorkTimeModelFields: budgetWorkTimeModelFields("Archiv", 1)})
	require.NoError(t, err)
	testpkg.OwnTestWorkTimeModel(t, db, model.ID)

	response, data := catalogRequest(t, router, http.MethodPost, fmt.Sprintf("/%d/archive", model.ID))
	require.Equal(t, http.StatusOK, response.StatusCode)
	var archived catalogRow
	require.NoError(t, json.Unmarshal(data, &archived))
	require.NotNil(t, archived.ArchivedAt)
	assert.NotNil(t, listCatalog(t, router)["Archiv"].ArchivedAt, "the catalog still lists the archived template")

	response, data = catalogRequest(t, router, http.MethodPost, fmt.Sprintf("/%d/restore", model.ID))
	require.Equal(t, http.StatusOK, response.StatusCode)
	var restored catalogRow
	require.NoError(t, json.Unmarshal(data, &restored))
	assert.Nil(t, restored.ArchivedAt)
	assert.Zero(t, notified, "archiving changes no Soll, so no time account is invalidated")

	foreign := testpkg.NewTenantScope(t, db)
	other, err := models.CreateWorkTimeModel(foreign.Context(), workforce.CreateWorkTimeModel{WorkTimeModelFields: budgetWorkTimeModelFields("Fremd", 1)})
	require.NoError(t, err)
	testpkg.OwnTestWorkTimeModel(t, db, other.ID)
	response, _ = catalogRequest(t, router, http.MethodPost, fmt.Sprintf("/%d/archive", other.ID))
	assert.Equal(t, http.StatusNotFound, response.StatusCode, "another school's template is not found")
}
