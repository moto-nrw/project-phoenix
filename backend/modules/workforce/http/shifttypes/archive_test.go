//go:build testhooks

package shifttypes_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shiftTypeWire struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	ArchivedAt *string `json:"archived_at"`
	Usage      *struct {
		Shifts int `json:"shifts"`
		Series int `json:"series"`
	} `json:"usage"`
}

// TestShiftTypeRemovalArchivesUsedTypes guards #6845 at the route: removing
// a type a planned shift carries archives it and keeps the shift's type, an
// unused type is deleted, and the list hides archived types unless the
// Dienstplan asks for them.
func TestShiftTypeRemovalArchivesUsedTypes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	types, references, _, err := api.NewShiftTypeTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := testpkg.MountedQueryBudgetRouter("/api/shift-types", api.NewShiftTypesRouterForBudgetTest(types, references))
	claims := testutil.DefaultTestClaims()
	claims.TenantID = testpkg.Tenant(t)
	claims.Permissions = []string{"time_tracking:manage"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)
	ctx := testpkg.Ctx(t)

	used, err := types.CreateShiftType(ctx, workforce.ShiftTypeInput{Name: "Betreuung", Color: "#83CD2D"})
	require.NoError(t, err)
	unused, err := types.CreateShiftType(ctx, workforce.ShiftTypeInput{Name: "Pause", Color: "#6B7280"})
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Archive", "Route")
	var shiftID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO schedule.staff_shifts (tenant_id, staff_id, date, start_time, end_time, break_minutes, shift_type_id, created_by)
		VALUES (?, ?, '2026-07-06', '08:00', '12:00', 0, ?, ?)
		RETURNING id
	`, testpkg.Tenant(t), staff.ID, used.ID, staff.ID).Scan(ctx, &shiftID))

	list := func(query string) []shiftTypeWire {
		t.Helper()
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/"+query, nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data []shiftTypeWire `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		return envelope.Data
	}
	remove := func(id int64) string {
		t.Helper()
		request := testutil.NewAuthenticatedRequest(t, http.MethodDelete, "/"+strconv.FormatInt(id, 10), nil, testutil.WithJWTBearer(token))
		response := testutil.ExecuteRequestForTest(t, router, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var envelope struct {
			Data struct {
				Outcome string `json:"outcome"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		return envelope.Data.Outcome
	}

	withUsage := list("?with_usage=true")
	require.Len(t, withUsage, 2)
	require.Equal(t, used.ID, withUsage[0].ID)
	require.NotNil(t, withUsage[0].Usage)
	assert.Equal(t, 1, withUsage[0].Usage.Shifts)
	require.NotNil(t, withUsage[1].Usage)
	assert.Zero(t, withUsage[1].Usage.Shifts)

	assert.Equal(t, "archived", remove(used.ID))
	assert.Equal(t, "deleted", remove(unused.ID))

	var shiftTypeID *int64
	require.NoError(t, db.NewRaw(`SELECT shift_type_id FROM schedule.staff_shifts WHERE id = ?`, shiftID).Scan(ctx, &shiftTypeID))
	require.NotNil(t, shiftTypeID, "the planned shift keeps its type")
	assert.Equal(t, used.ID, *shiftTypeID)

	assert.Empty(t, list(""), "the choice hides the archived type and the deleted one is gone")
	all := list("?include_archived=true")
	require.Len(t, all, 1)
	assert.Equal(t, used.ID, all[0].ID)
	assert.NotNil(t, all[0].ArchivedAt)
}
