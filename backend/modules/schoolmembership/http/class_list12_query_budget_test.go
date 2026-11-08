package staff_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

type classList12Response struct {
	Data []struct {
		ID        string   `json:"id"`
		FirstName string   `json:"first_name"`
		Matching  []string `json:"matching_student_ids"`
	} `json:"data"`
}

func TestClassList12FullHTTPQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	router, err := api.ClassList12Router(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "class-list12-budget@test.local")
	claims := testutil.Claims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: testpkg.Tenant(t)}
	expected := map[string][]string{}
	counter := testpkg.CaptureQueriesForContext(t, db)
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("Child%d", i)
		entry := testpkg.CreateTestClassListEntry(t, db, name, "List", "12a")
		student := testpkg.CreateTestStudent(t, db, name, "List", "12a")
		expected[strconv.FormatInt(entry.ID, 10)] = []string{strconv.FormatInt(student.ID, 10)}
		if i != 2 && i != 7 {
			continue
		}
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(counter.Context(req.Context()))
		rec := testutil.ExecuteWithAuthPermissions(t, testpkg.MountedQueryBudgetRouter("/api/class-list-entries", router), req, claims, []string{"users:read"})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var payload classList12Response
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Len(t, payload.Data, i+1)
		for j, entry := range payload.Data {
			require.Equal(t, fmt.Sprintf("Child%d", j), entry.FirstName, "display order")
			require.Equal(t, expected[entry.ID], entry.Matching, "each entry retains its own match")
		}
		if i == 7 {
			querybudgets.AssertQueryBudget(t, "api.class_list_entries.list.full.n8", counter.Measurement(counter.Queries()))
		} else {
			querybudgets.AssertQueryBudget(t, "api.class_list_entries.list.full.n3", counter.Measurement(counter.Queries()))
		}
	}
}
