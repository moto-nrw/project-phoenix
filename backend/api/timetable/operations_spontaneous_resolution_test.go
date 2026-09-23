package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOperationsCreateAndStartSpontaneousResolvesActivityAgainstRealRepositories
// runs the spontaneous-start activity resolution against the real room,
// activity group and category repositories (#3557). Their not-found errors
// carry the repository not-found sentinel, not sql.ErrNoRows, so a new title
// and a tenant without the "Spontan" category must still create both instead
// of failing with 500.
func TestOperationsCreateAndStartSpontaneousResolvesActivityAgainstRealRepositories(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := mustTimetableTestRepositories(db)
	room := testpkg.CreateTestRoom(t, db, "Spontanraum")
	staff := testpkg.CreateTestStaff(t, db, "Spontan", "Starter")
	existingGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Bestehende AG %d", time.Now().UnixNano()))

	instance := &schedule.ActivityInstance{Status: schedule.InstanceStatusActive}
	service := &fakeOperationsService{start: &timetableplanning.StartInstanceResult{Instance: instance}}
	res := NewResource(Dependencies{
		OperationsService: service,
		TimetableData: timetableplanning.NewTimetableDataService(timetableplanning.TimetableDataDependencies{
			ActiveGroupRepo:      &fakeOperationActiveGroupRepo{},
			RoomRepo:             repos.Room,
			ActivityGroupRepo:    repos.ActivityGroup,
			ActivityCategoryRepo: repos.ActivityCategory,
			DB:                   db,
		}),
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
				person := &userModels.Person{}
				person.ID = staff.PersonID
				return person, nil
			},
			GetStaffByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
				return staff, nil
			},
		},
		SettingsService: &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithTenantID(testpkg.WithPackageTenantRuntime(r.Context()), testpkg.Tenant(t))))
		})
	})
	router.Use(testpkg.TenantTxMiddleware(db))
	router.Post("/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	post := func(title string, roomID int64) *httptest.ResponseRecorder {
		t.Helper()
		service.lastSpontaneousInput = nil
		body, err := json.Marshal(map[string]any{"title": title, "room_id": roomID})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/spontaneous/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testutil.WithClaims(t, testutil.AdminTestClaims(120))(req)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}
	start := func(title string) int64 {
		t.Helper()
		rr := post(title, room.ID)
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
		require.NotNil(t, service.lastSpontaneousInput)
		require.NotNil(t, service.lastSpontaneousInput.ActivityGroupID)
		return *service.lastSpontaneousInput.ActivityGroupID
	}

	_, err := repos.ActivityCategory.FindByNameIncludingArchivedForShare(testpkg.Ctx(t), "Spontan")
	require.ErrorIs(t, err, activityModels.ErrNotFound, "the tenant starts without a Spontan category")

	newTitle := fmt.Sprintf("Neue Werkstatt %d", time.Now().UnixNano())
	createdGroupID := start(newTitle)

	assert.NotEqual(t, existingGroup.ID, createdGroupID)
	createdGroup, err := repos.ActivityGroup.FindByID(testpkg.Ctx(t), createdGroupID)
	require.NoError(t, err)
	assert.Equal(t, newTitle, createdGroup.Name)
	category, err := repos.ActivityCategory.FindByNameIncludingArchivedForShare(testpkg.Ctx(t), "Spontan")
	require.NoError(t, err, "a tenant without the Spontan category gets one on the first spontaneous start")
	assert.Equal(t, category.ID, createdGroup.CategoryID)

	assert.Equal(t, createdGroupID, start(newTitle), "a repeated title reuses the group created before")
	assert.Equal(t, existingGroup.ID, start(existingGroup.Name), "an existing title reuses that group")

	deletedRoom := testpkg.CreateTestRoom(t, db, "Geloeschter Raum")
	require.NoError(t, repos.Room.Delete(testpkg.Ctx(t), deletedRoom.ID))
	rr := post(newTitle, deletedRoom.ID)
	assert.Equal(t, http.StatusBadRequest, rr.Code, "a missing room is a client error, not a 500")
	assert.Nil(t, service.lastSpontaneousInput)
}
