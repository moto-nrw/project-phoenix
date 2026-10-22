package httpadapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func setupRoomCategoriesRoute(t *testing.T) (chi.Router, *bun.DB) {
	t.Helper()
	db, _, protectedRoutes, _ := ComposeRooms(t)
	categories, err := facilitiesCompose.NewRoomCategories(facilitiesCompose.Dependencies{
		DB: db, Observe: func(facilitiesCompose.Observation) {},
	})
	require.NoError(t, err)
	return NewCategoryResource(categories, protectedRoutes).Router(), db
}

func roomCategoryRequest(method, path, body string) *http.Request {
	req := testutil.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Code
}

func TestRoomCategoriesRoute(t *testing.T) {
	t.Parallel()
	router, db := setupRoomCategoriesRoute(t)
	claims := testutil.DefaultTestClaims()
	write := []string{"rooms:read", "rooms:update"}

	rr := testutil.ExecuteWithAuthPermissions(t, router,
		roomCategoryRequest(http.MethodPost, "/", `{"name":"Sport","color":"#827710"}`), claims, []string{"rooms:read"})
	routetest.AssertErrorResponse(t, rr, http.StatusForbidden)

	rr = testutil.ExecuteWithAuthPermissions(t, router,
		roomCategoryRequest(http.MethodPost, "/", `{"name":"Sport","color":"#827710"}`), claims, write)
	require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())
	var created struct {
		Data RoomCategoryResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	assert.Equal(t, "Sport", created.Data.Name)
	assert.Equal(t, "#827710", created.Data.Color)

	rr = testutil.ExecuteWithAuthPermissions(t, router,
		roomCategoryRequest(http.MethodPost, "/", `{"name":"sport","color":"#374750"}`), claims, write)
	routetest.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Equal(t, "rooms.category_duplicate", errorCode(t, rr.Body.Bytes()))

	rr = testutil.ExecuteWithAuthPermissions(t, router,
		roomCategoryRequest(http.MethodPost, "/", `{"name":"Mensa","color":"#D946EF"}`), claims, write)
	routetest.AssertErrorResponse(t, rr, http.StatusBadRequest)

	room := testpkg.CreateTestRoom(t, db, "Sporthalle")
	_, err := db.NewUpdate().TableExpr("facilities.rooms").
		Set("category = 'Sport'").Where("id = ?", room.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	path := "/" + itoa(created.Data.ID)
	rr = testutil.ExecuteWithAuthPermissions(t, router, roomCategoryRequest(http.MethodDelete, path, ""), claims, write)
	routetest.AssertErrorResponse(t, rr, http.StatusConflict)
	assert.Equal(t, "rooms.category_in_use", errorCode(t, rr.Body.Bytes()))

	rr = testutil.ExecuteWithAuthPermissions(t, router, roomCategoryRequest(http.MethodGet, "/", ""), claims, []string{"rooms:read"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	var listed struct {
		Data []RoomCategoryResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &listed))
	require.Len(t, listed.Data, 1)
	assert.Equal(t, 1, listed.Data[0].RoomCount)

	rr = testutil.ExecuteWithAuthPermissions(t, router,
		roomCategoryRequest(http.MethodPut, path, `{"name":"Bewegung","color":"#827710"}`), claims, write)
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	_, err = db.NewUpdate().TableExpr("facilities.rooms").
		Set("category = NULL").Where("id = ?", room.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	rr = testutil.ExecuteWithAuthPermissions(t, router, roomCategoryRequest(http.MethodDelete, path, ""), claims, write)
	require.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

	rr = testutil.ExecuteWithAuthPermissions(t, router,
		roomCategoryRequest(http.MethodPut, path, `{"name":"Bewegung","color":"#827710"}`), claims, write)
	routetest.AssertErrorResponse(t, rr, http.StatusNotFound)
	assert.Equal(t, "rooms.category_not_found", errorCode(t, rr.Body.Bytes()))
}

// The catalog reads its categories and the room count per category in one
// statement each, however many of both a school has (Rule 15).
func TestRoomCategoriesListQueryBudget(t *testing.T) {
	t.Parallel()
	router, db := setupRoomCategoriesRoute(t)
	claims := testutil.DefaultTestClaims()
	add := func(from, to int) {
		for i := from; i < to; i++ {
			name := fmt.Sprintf("Budget %d", i)
			rr := testutil.ExecuteWithAuthPermissions(t, router,
				roomCategoryRequest(http.MethodPost, "/", fmt.Sprintf(`{"name":%q,"color":"#374750"}`, name)),
				claims, []string{"rooms:update"})
			require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())
			room := testpkg.CreateTestRoom(t, db, "Raum "+name)
			_, err := db.NewUpdate().TableExpr("facilities.rooms").
				Set("category = ?", name).Where("id = ?", room.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueriesForContext(t, db)
	run := func() []string {
		counter.Reset()
		req := roomCategoryRequest(http.MethodGet, "/", "")
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"rooms:read"})
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	t.Logf("query budget: 3 categories -> %d reads, 8 categories -> %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "api.room_categories.list.reads", large)
}
