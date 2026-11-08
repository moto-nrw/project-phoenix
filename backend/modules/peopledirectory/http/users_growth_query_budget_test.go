//go:build testhooks

package users_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestUsersProductionBoundPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	identity, err := api.NewAuthTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	people := api.NewStudentHTTPTestPeople(database)
	resource := api.NewUsersResource(people, identity.Auth.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, lookupErr := identity.Auth.LookupRFIDCard(ctx, tagID)
		return found, lookupErr
	})
	router := testutil.NewTenantRouter(db)
	router.Mount("/api/users", resource.Router())
	reader := testpkg.CreateTestAccount(t, db, "person-list-reader")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), reader.ID, "users:read")
	token, _, err := identity.Auth.IssueTokensForAuthenticatedAccount(testpkg.Ctx(t), reader.ID, testpkg.Tenant(t), "", "")
	require.NoError(t, err)
	claims, err := testutil.TestTokenAuth(t).ParseAccessJWT(token)
	require.NoError(t, err)
	require.False(t, claims.IsAdmin)
	require.Contains(t, claims.Permissions, "users:read")

	prefix := "BudgetUsers"
	foreign := testpkg.NewTenantScope(t, db)
	testpkg.CreateTestPersonForTenant(t, db, foreign.TenantID, prefix+"Foreign", "Hidden")
	want := []users.PersonResponse{}
	counter := testpkg.CaptureQueries(t, db)
	counter.Stop()
	for _, n := range []int{3, 8} {
		for len(want) < n {
			account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("person-list-%d", len(want)))
			person := testpkg.CreateTestPersonWithAccountID(t, db, prefix+fmt.Sprint(len(want)), "Visible", account.ID)
			card := testpkg.CreateTestRFIDCard(t, db, fmt.Sprintf("BUDGET%d", len(want)))
			testpkg.LinkRFIDToStudent(t, db, person.ID, card.ID)
			// The tag assignment updates the persisted version shown in the DTO.
			require.NoError(t, db.NewSelect().Model(person).ModelTableExpr("users.persons AS person").Where("id = ?", person.ID).Scan(context.Background()))
			want = append(want, users.PersonResponse{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
				Email: account.Email, TagID: card.ID, AccountID: account.ID, CreatedAt: person.CreatedAt.UTC(), UpdatedAt: person.UpdatedAt.UTC()})
		}
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/users/?first_name="+prefix+"&page=1&page_size=20", nil,
			testutil.WithJWTBearer(token))
		counter.Reset()
		counter.Start()
		response := testutil.ExecuteRequest(router, request)
		counter.Stop()
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Status     string                 `json:"status"`
			Message    string                 `json:"message"`
			Data       []users.PersonResponse `json:"data"`
			Pagination struct {
				CurrentPage  int `json:"current_page"`
				PageSize     int `json:"page_size"`
				TotalPages   int `json:"total_pages"`
				TotalRecords int `json:"total_records"`
			} `json:"pagination"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		require.Equal(t, "success", envelope.Status)
		require.Equal(t, "Persons retrieved successfully", envelope.Message)
		require.Len(t, envelope.Data, n)
		for i := range envelope.Data {
			envelope.Data[i].CreatedAt = envelope.Data[i].CreatedAt.UTC()
			envelope.Data[i].UpdatedAt = envelope.Data[i].UpdatedAt.UTC()
		}
		require.ElementsMatch(t, want, envelope.Data)
		require.Equal(t, 1, envelope.Pagination.CurrentPage)
		require.Equal(t, 20, envelope.Pagination.PageSize)
		require.Equal(t, 1, envelope.Pagination.TotalPages)
		require.Equal(t, n, envelope.Pagination.TotalRecords)
		require.Zero(t, counter.WriteRows())
		t.Logf("users positive N%d statements=%d", n, counter.Total())
		querybudgets.AssertQueryBudget(t, "api.users.list.positive_growth", counter.Measurement(counter.Queries()))
	}
}
