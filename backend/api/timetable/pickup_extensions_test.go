// Integration tests for the later-pickup block decisions (#3261):
// GET /pickup-extensions and POST /pickup-extensions/{id}/resolve. The
// handlers run in the tenant transaction like production, with permissions
// injected into the request context.
package timetable

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const pickupExtensionDate = "2099-03-03"

type pickupExtensionSetup struct {
	db       *bun.DB
	ctx      context.Context
	res      *Resource
	module   timetable.Capability
	room     int64
	staff    int64
	freePlay int64
	other    int64
}

func buildPickupExtensionSetup(t *testing.T, db *bun.DB) *pickupExtensionSetup {
	t.Helper()
	suffix := time.Now().UnixNano()
	module := mustTimetableTestRepositories(db).Timetable
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("PE-Room-%d", suffix))
	staff := testpkg.CreateTestStaff(t, db, "Swantje", fmt.Sprintf("PE-%d", suffix))
	other := testpkg.CreateTestStudent(t, db, "Ben", fmt.Sprintf("PE-Other-%d", suffix), "1a")
	freePlay := testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2099, time.March, 3), room.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:45", EndHHMM: "16:00", Title: "Freies Spiel",
	})
	testpkg.CreateTestInstanceStudent(t, db, freePlay.ID, other.ID, timetable.SlotAttendanceExpected)
	res := NewResource(Dependencies{
		PickupExtensions: module,
		RecurrenceLock:   repositories.MustNewTimetableRecurrenceLock(db),
		PersonService: usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
			StudentRepo: repositories.NewStudentRepository(db),
			PersonRepo:  usersRepo.NewPersonRepository(db),
			StaffRepo:   mustTimetableTestRepositories(db).Staff,
		}),
		DB: db,
	})
	return &pickupExtensionSetup{
		db: db, ctx: testpkg.Ctx(t), res: res, module: module,
		room: room.ID, staff: staff.ID, freePlay: freePlay.ID, other: other.ID,
	}
}

// addChild creates a child whose Tuesday pickup moved from 14:45 to 16:00.
func (s *pickupExtensionSetup) addChild(t *testing.T, first string) (studentID int64, name string) {
	t.Helper()
	last := fmt.Sprintf("PE-Child-%d", time.Now().UnixNano())
	student := testpkg.CreateTestStudent(t, s.db, first, last, "1a")
	exception := testpkg.CreateTestPickupException(t, s.db, student.ID, testpkg.Date(2099, time.March, 3), s.staff, "16:00", "")
	require.NoError(t, s.module.RecordPickupDayExtension(s.ctx, timetable.PickupDayExtension{
		StudentID: student.ID, PickupExceptionID: exception.ID, Date: pickupExtensionDate,
		PreviousPickup: "14:45", Pickup: "16:00",
	}))
	return student.ID, first + " " + last
}

func (s *pickupExtensionSetup) router(perms []string) chi.Router {
	tenantID := tenant.FromContext(s.ctx)
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(req.Context()), tenantID)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Use(testpkg.TenantTxMiddleware(s.db))
	r.Get("/pickup-extensions", s.res.listPickupExtensions)
	r.Post("/pickup-extensions/{id}/resolve", s.res.resolvePickupExtension)
	return r
}

func TestPickupExtensions_ListAndResolve(t *testing.T) {
	t.Parallel()
	s := buildPickupExtensionSetup(t, testpkg.SetupTestDB(t))
	childID, childName := s.addChild(t, "Mia")
	router := s.router([]string{"admin:*"})

	w := executeRequest(router, http.MethodGet, fmt.Sprintf("/pickup-extensions?student_id=%d", childID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	listed := decodeTemplateData[pickupExtensionsResponse](t, w)
	require.Len(t, listed.Tasks, 1)
	task := listed.Tasks[0]
	assert.Equal(t, childID, task.StudentID)
	assert.Equal(t, childName, task.StudentName)
	assert.Equal(t, timetable.PickupExtensionKindDay, task.Kind)
	assert.Equal(t, pickupExtensionDate, task.Date)
	assert.Equal(t, "14:45", task.PreviousPickupTime)
	assert.Equal(t, "16:00", task.PickupTime)
	assert.Equal(t, []pickupExtensionBlockResponse{{ID: s.freePlay, Title: "Freies Spiel", StartTime: "14:45", EndTime: "16:00"}}, task.Blocks)

	w = executeRequest(router, http.MethodPost, fmt.Sprintf("/pickup-extensions/%d/resolve", task.ID),
		map[string]any{"block_ids": []int64{s.freePlay}})
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	resolved := decodeTemplateData[resolvePickupExtensionResponse](t, w)
	assert.Equal(t, childID, resolved.StudentID)
	assert.Equal(t, 1, resolved.InstanceCount)
	require.Len(t, resolved.AssignedBlocks, 1)
	assert.Equal(t, s.freePlay, resolved.AssignedBlocks[0].ID)

	count, err := s.db.NewSelect().TableExpr("schedule.instance_students").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("instance_id = ?", s.freePlay).Where("student_id = ?", childID).
		Count(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
	assert.Equal(t, 1, count, "the team sees the child on the block's list")

	w = executeRequest(router, http.MethodGet, fmt.Sprintf("/pickup-extensions?student_id=%d", childID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, decodeTemplateData[pickupExtensionsResponse](t, w).Tasks)

	w = executeRequest(router, http.MethodPost, fmt.Sprintf("/pickup-extensions/%d/resolve", task.ID), map[string]any{})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), pickupExtensionNotFoundCode)
}

func TestPickupExtensions_ResolveErrors(t *testing.T) {
	t.Parallel()
	s := buildPickupExtensionSetup(t, testpkg.SetupTestDB(t))
	childID, _ := s.addChild(t, "Lea")
	router := s.router([]string{"admin:*"})
	tasks, err := s.module.ListOpenPickupExtensions(s.ctx, childID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	path := fmt.Sprintf("/pickup-extensions/%d/resolve", tasks[0].ID)

	office := testpkg.CreateTestActivityInstance(t, s.db, testpkg.Date(2099, time.March, 3), s.room, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "15:30", Title: "Bürotermin",
	})
	w := executeRequest(router, http.MethodPost, path, map[string]any{"block_ids": []int64{office.ID}})
	assert.Equal(t, http.StatusConflict, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), pickupExtensionBlockGoneCode)

	w = executeRequest(router, http.MethodPost, path, map[string]any{"block_ids": []int64{-4}})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	w = executeRequest(router, http.MethodGet, "/pickup-extensions?student_id=abc", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())

	// The failed attempts changed nothing; the task is still open.
	tasks, err = s.module.ListOpenPickupExtensions(s.ctx, childID)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestPickupExtensions_UnreadableChildrenAreHidden(t *testing.T) {
	t.Parallel()
	s := buildPickupExtensionSetup(t, testpkg.SetupIsolatedTestDB(t))
	childID, _ := s.addChild(t, "Ole")
	tasks, err := s.module.ListOpenPickupExtensions(s.ctx, childID)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// schedules:manage reaches the route, but without a verified staff
	// context no child is readable: no names leak, and no roster changes.
	router := s.router([]string{"schedules:manage"})
	w := executeRequest(router, http.MethodGet, "/pickup-extensions", nil)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	assert.Empty(t, decodeTemplateData[pickupExtensionsResponse](t, w).Tasks)

	counter := testpkg.CaptureQueries(t, s.db)
	counter.Reset()
	w = executeRequest(router, http.MethodPost, fmt.Sprintf("/pickup-extensions/%d/resolve", tasks[0].ID),
		map[string]any{"block_ids": []int64{s.freePlay}})
	assert.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	taskQuerySeen := false
	for _, query := range counter.Queries() {
		normalized := strings.ToUpper(query)
		if strings.Contains(normalized, "SCHEDULE.PICKUP_EXTENSION_TASKS") {
			taskQuerySeen = true
			assert.NotContains(t, normalized, "FOR UPDATE", "unreadable students must be rejected before resolve locks the task")
		}
	}
	assert.True(t, taskQuerySeen, "the task must be read to authorize its student")
	tasks, err = s.module.ListOpenPickupExtensions(s.ctx, childID)
	require.NoError(t, err)
	assert.Len(t, tasks, 1, "the refused resolve rolled back")
}

// TestPickupExtensionsListQueryBudget pins GET /pickup-extensions: the task
// read, one batched block read per task kind, and two name reads, flat in
// the number of open tasks.
func TestPickupExtensionsListQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	s := buildPickupExtensionSetup(t, db)
	router := s.router([]string{"admin:*"})

	created := 0
	addChildren := func(n int) {
		for range n {
			s.addChild(t, fmt.Sprintf("Kind%d", created))
			created++
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	run := func() int {
		counter.Reset()
		w := executeRequest(router, http.MethodGet, "/pickup-extensions", nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		require.Len(t, decodeTemplateData[pickupExtensionsResponse](t, w).Tasks, created)
		return counter.Total()
	}

	addChildren(3)
	smallCount := run()
	addChildren(3)
	largeCount := run()

	t.Logf("query budget: 3 tasks → %d queries, 6 tasks → %d queries", smallCount, largeCount)
	assert.Equal(t, smallCount, largeCount, "query count must not grow with the number of tasks")
	testpkg.AssertQueryBudget(t, "api.timetable.pickup_extensions.list", counter.Queries())
}
