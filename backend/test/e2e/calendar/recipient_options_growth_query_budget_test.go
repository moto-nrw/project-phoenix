package calendar_e2e_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

type recipientOptionWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestCalendarRecipientOptionsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, router := setupPersonalCalendarRoute(t)
	_, account := testpkg.CreateTestCalendarStaff(t, db, "Calendar", "Organizer")
	token := calendarToken(t, account.ID, permissions.CalendarManage)
	ids := []string{}
	for _, n := range []int{3, 8} {
		for len(ids) < n {
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ids = append(ids, strconv.FormatInt(chain.GuardianProfileID, 10))
		}
		counter := testpkg.CaptureQueries(t, db)
		rec := doJSON(t, testpkg.MountedQueryBudgetRouter("/api", router), http.MethodGet, "/calendar/recipient-options?q=schneider&limit=50", token, nil)
		queries := counter.Measurement(counter.Queries())
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var out struct {
			Data struct {
				Staff    []recipientOptionWire `json:"staff"`
				Parents  []recipientOptionWire `json:"parents"`
				Groups   []recipientOptionWire `json:"groups"`
				Classes  []string              `json:"classes"`
				Students []struct {
					ID          string  `json:"id"`
					Name        string  `json:"name"`
					SchoolClass string  `json:"school_class"`
					GroupID     *string `json:"group_id,omitempty"`
				} `json:"students"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
		got := make([]string, 0, len(out.Data.Parents))
		for _, parent := range out.Data.Parents {
			got = append(got, parent.ID)
			require.NotEmpty(t, parent.Name)
		}
		require.ElementsMatch(t, ids, got)
		t.Logf("full calendar recipient-options N%d statements=%d", n, len(queries.Statements))
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "api.calendar.recipient_options.parents_3", queries)
		} else {
			querybudgets.AssertQueryBudget(t, "api.calendar.recipient_options.parents_8", queries)
		}
	}
}
