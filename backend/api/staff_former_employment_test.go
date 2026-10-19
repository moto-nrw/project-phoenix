package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffListFormerEmploymentsAgainstTheDatabase drives the Ehemalige
// filter (#3403) through the composed /staff router: retired memberships of
// the school come back with their period, current staff does not, and the
// statement count stays flat when more people leave (Rule 15).
func TestStaffListFormerEmploymentsAgainstTheDatabase(t *testing.T) {
	t.Parallel()

	ctx := setupStaffCompositionRoute(t)
	counter := ctx.countQueries()
	token := staffCompositionToken(t, betreuerPermissions...)
	leftOn := calendar.NewDate(2026, 7, 31)

	currentID, _ := ctx.createStaff("Aktuelle", "Kollegin")
	retired := map[string]bool{}
	retireOne := func(n int) {
		staffID, _ := ctx.createStaff(fmt.Sprintf("Ehemalige%d", n), "Kollegin")
		ctx.retire(staffID, leftOn.BerlinMidnight().Add(10*time.Hour))
		retired[strconv.FormatInt(staffID, 10)] = true
	}

	type row struct {
		ID        string `json:"id"`
		FirstName string `json:"first_name"`
		StartedOn string `json:"started_on"`
		EndedOn   string `json:"ended_on"`
	}
	list := func() ([]row, []string) {
		counter.Reset()
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/staff?employment=former", nil, testutil.WithJWTBearer(token))
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteRequest(ctx.router, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data []row `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		return body.Data, counter.Queries()
	}
	ours := func(rows []row) []row {
		result := []row{}
		for _, r := range rows {
			if retired[r.ID] || r.ID == strconv.FormatInt(currentID, 10) {
				result = append(result, r)
			}
		}
		return result
	}

	retireOne(1)
	small, smallQueries := list()
	retireOne(2)
	retireOne(3)
	large, largeQueries := list()

	require.Len(t, ours(small), 1)
	require.Len(t, ours(large), 3, "every retired membership of the school is listed, current staff is not")
	for _, r := range ours(large) {
		assert.True(t, retired[r.ID])
		assert.Equal(t, leftOn.String(), r.EndedOn, "the period ends on the Berlin day the person left")
		assert.NotEmpty(t, r.StartedOn)
	}
	assert.Len(t, largeQueries, len(smallQueries), "flat in the number of former employments")
	testpkg.AssertQueryBudget(t, "modules.schoolmembership.staff.list_former", largeQueries)
}
