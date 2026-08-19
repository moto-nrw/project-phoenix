package timetable

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOperationsPlannedNow(t *testing.T) {
	service := &fakeOperationsService{
		planned: []scheduleSvc.OperationPlannedInstance{{ID: 220, Title: "Lernzeit"}},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)

	rr := executeOperationRequest(router, http.MethodGet, "/planned-now?date=2026-05-10", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(120), service.lastAccountID)
	assert.True(t, service.lastIsAdmin)
	assert.Equal(t, "2026-05-10", service.lastDate.Format(dateLayout))
	assert.Contains(t, rr.Body.String(), `"instances"`)

	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?horizon_minutes=480&limit=5&include_roster=true", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 480, service.lastPlannedOptions.HorizonMinutes)
	assert.Equal(t, 5, service.lastPlannedOptions.Limit)
	assert.True(t, service.lastPlannedOptions.IncludeRoster)

	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?scope=past", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, scheduleSvc.PlannedNowScopePast, service.lastPlannedOptions.Scope)
}

func testWorkdayNow() time.Time {
	return time.Date(2026, time.May, 11, 14, 0, 0, 0, timezone.Berlin)
}

func TestOperationsPlannedNowValidationAndWiring(t *testing.T) {
	res := NewResource(Dependencies{})
	router := operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)
	rr := executeOperationRequest(router, http.MethodGet, "/planned-now", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	res = NewResource(Dependencies{OperationsService: &fakeOperationsService{}})
	router = operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)
	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?date=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?horizon_minutes=-1", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?include_roster=maybe", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?scope=future", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?scope=past&date=2000-01-01", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsInstanceEndpoints(t *testing.T) {
	service := &fakeOperationsService{
		roster: &scheduleSvc.OperationRoster{Instance: scheduleSvc.OperationRosterInstance{ID: 230}},
		start: &scheduleSvc.StartInstanceResult{
			Instance:      &schedule.ActivityInstance{Status: schedule.InstanceStatusActive},
			ActiveGroupID: 330,
		},
		complete: &schedule.ActivityInstance{Status: schedule.InstanceStatusCompleted},
	}
	service.start.Instance.ID = 231
	service.complete.ID = 232
	res := NewResource(Dependencies{OperationsService: service})

	cases := []struct {
		name   string
		method string
		path   string
		fn     http.HandlerFunc
		body   any
	}{
		{"roster", http.MethodGet, "/instances/230/roster", res.operationsRoster, nil},
		{"start", http.MethodPost, "/instances/231/start", res.operationsStart, nil},
		{"complete", http.MethodPost, "/instances/232/complete", res.operationsComplete, map[string]any{"confirmed_present_student_ids": []int64{}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := operationRouter(tc.method, "/instances/{id}/"+lastPathSegment(tc.path), tc.fn)

			rr := executeOperationRequest(router, tc.method, tc.path, tc.body)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
	assert.Equal(t, int64(232), service.lastInstanceID)
}

func TestOperationsReopenEffectiveAdminScope(t *testing.T) {
	started := &schedule.ActivityInstance{Status: schedule.InstanceStatusActive}
	started.ID = 231
	service := &fakeOperationsService{
		start: &scheduleSvc.StartInstanceResult{
			Instance:      started,
			ActiveGroupID: 341,
		},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodPost, "/instances/{id}/reopen", res.operationsReopen)

	cases := []struct {
		name        string
		isAdmin     bool
		permissions []string
		wantAdmin   bool
	}{
		{name: "wildcard admin without admin role", permissions: []string{"admin:*"}, wantAdmin: true},
		{name: "full-access wildcard", permissions: []string{"*:*"}, wantAdmin: true},
		{name: "literal admin role", isAdmin: true, permissions: []string{"schedules:manage"}, wantAdmin: true},
		{name: "ordinary staff", permissions: []string{"schedules:manage"}, wantAdmin: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service.lastIsAdmin = false
			req := httptest.NewRequest(http.MethodPost, "/instances/231/reopen", nil)
			testutil.WithClaims(jwt.AppClaims{ID: 120, IsAdmin: tc.isAdmin, TenantID: 1})(req)
			testutil.WithPermissions(tc.permissions...)(req)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
			assert.Equal(t, int64(120), service.lastAccountID)
			assert.Equal(t, tc.wantAdmin, service.lastIsAdmin)
		})
	}
}

func TestOperationsCreateAndStartSpontaneous(t *testing.T) {
	createdInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusPlanned}
	createdInstance.ID = 241
	startedInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusActive}
	startedInstance.ID = 241
	instanceSvc := &mockInstanceService{
		createRes: createdInstance,
	}
	service := &fakeOperationsService{
		start: &scheduleSvc.StartInstanceResult{
			Instance:      startedInstance,
			ActiveGroupID: 341,
		},
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: service,
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(_ context.Context, accountID int64) (*userModels.Person, error) {
				assert.Equal(t, int64(120), accountID)
				person := &userModels.Person{}
				person.ID = 220
				return person, nil
			},
			GetStaffByPersonIDFn: func(_ context.Context, personID int64) (*userModels.Staff, error) {
				assert.Equal(t, int64(220), personID)
				staff := &userModels.Staff{}
				staff.ID = 320
				return staff, nil
			},
		},
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)
	roomID := int64(70)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":             "Freispiel",
		"room_id":           roomID,
		"activity_group_id": int64(71),
		"staff_ids":         []int64{321},
	})

	require.Equal(t, http.StatusCreated, rr.Code)
	require.NotNil(t, service.lastSpontaneousInput)
	assert.Equal(t, roomID, service.lastSpontaneousInput.RoomID)
	require.NotNil(t, service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, int64(71), *service.lastSpontaneousInput.ActivityGroupID)
	require.NotNil(t, service.lastSpontaneousInput.IsSpontaneous)
	assert.True(t, *service.lastSpontaneousInput.IsSpontaneous)
	assert.Equal(t, []int64{321, 320}, service.lastSpontaneousInput.StaffIDs)
	assert.Empty(t, service.lastSpontaneousInput.StudentIDs)
	assert.Equal(t, int64(241), service.lastInstanceID)
	assert.Contains(t, rr.Body.String(), `"active_group_id":341`)
}

func TestOperationsCreateAndStartSpontaneousRollsBackNon5xxFailures(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	guardianRepo := repositories.NewFactory(db).GuardianProfile
	email := fmt.Sprintf("spontaneous-start-rollback-%d@test.local", time.Now().UnixNano())
	probe := &userModels.GuardianProfile{
		FirstName:              "Spontaneous",
		LastName:               "Rollback",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	t.Cleanup(func() {
		if probe.ID > 0 {
			_ = guardianRepo.Delete(testpkg.TenantContext(1), probe.ID)
		}
	})

	createdInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusPlanned}
	createdInstance.ID = 244
	// Start fails non-5xx: the real service's Start delegates to InstanceService.Start,
	// which returns an invalid transition (→ 409). The Create write made just before
	// must roll back.
	instanceSvc := &rollbackProbeInstanceService{
		mockInstanceService: &mockInstanceService{startErr: scheduleSvc.ErrInvalidInstanceTransition},
		create: func(ctx context.Context, _ scheduleSvc.CreateInstanceInput) (*schedule.ActivityInstance, error) {
			require.NoError(t, guardianRepo.Create(ctx, probe))
			require.Greater(t, probe.ID, int64(0), "probe write must happen inside the tenant transaction")
			return createdInstance, nil
		},
	}
	personSvc := &userstest.PersonServiceMock{
		FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
			person := &userModels.Person{}
			person.ID = 224
			return person, nil
		},
		GetStaffByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
			staff := &userModels.Staff{}
			staff.ID = 324
			return staff, nil
		},
	}
	settings := &fakeOperationSettingsService{hasOverride: true, boolValue: true}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: newRealSpontaneousOpsService(db, instanceSvc, personSvc, settings),
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		PersonService:     personSvc,
		SettingsService:   settings,
	})
	res.Now = testWorkdayNow

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithTenantID(r.Context(), 1)))
		})
	})
	router.Use(tenant.TenantTxMiddleware(db))
	router.Post("/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	body, err := json.Marshal(map[string]any{
		"title":             "Freispiel",
		"room_id":           int64(70),
		"activity_group_id": int64(71),
	})
	require.NoError(t, err)
	execute := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/spontaneous/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testutil.WithClaims(testutil.AdminTestClaims(120))(req)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	rr := execute()

	assert.Equal(t, http.StatusConflict, rr.Code)
	_, err = guardianRepo.FindByID(testpkg.TenantContext(1), probe.ID)
	assert.ErrorIs(t, err, userModels.ErrGuardianProfileNotFound,
		"a non-5xx start failure must roll back writes made before Start")

	createEmail := fmt.Sprintf("spontaneous-create-rollback-%d@test.local", time.Now().UnixNano())
	createProbe := &userModels.GuardianProfile{
		FirstName:              "Spontaneous",
		LastName:               "CreateRollback",
		Email:                  &createEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	t.Cleanup(func() {
		if createProbe.ID > 0 {
			_ = guardianRepo.Delete(testpkg.TenantContext(1), createProbe.ID)
		}
	})
	// Create now fails non-5xx before Start is reached; the wrapping in
	// SpontaneousCreateError keeps the create-specific 400 mapping, and the
	// activity-resolution write must roll back.
	instanceSvc.create = func(ctx context.Context, _ scheduleSvc.CreateInstanceInput) (*schedule.ActivityInstance, error) {
		require.NoError(t, guardianRepo.Create(ctx, createProbe))
		return nil, fmt.Errorf("%w: invalid staff_ids", scheduleSvc.ErrInvalidInstanceReference)
	}

	rr = execute()

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	_, err = guardianRepo.FindByID(testpkg.TenantContext(1), createProbe.ID)
	assert.ErrorIs(t, err, userModels.ErrGuardianProfileNotFound,
		"a non-5xx create failure must roll back activity resolution writes")
}

type rollbackProbeInstanceService struct {
	*mockInstanceService
	create func(context.Context, scheduleSvc.CreateInstanceInput) (*schedule.ActivityInstance, error)
}

func (s *rollbackProbeInstanceService) Create(ctx context.Context, req scheduleSvc.CreateInstanceInput) (*schedule.ActivityInstance, error) {
	return s.create(ctx, req)
}

func TestOperationsCreateAndStartSpontaneousReusesActivityByName(t *testing.T) {
	createdInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusPlanned}
	createdInstance.ID = 242
	startedInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusActive}
	startedInstance.ID = 242
	instanceSvc := &mockInstanceService{createRes: createdInstance}
	groupRepo := &fakeOperationActivityGroupRepo{
		findByNameResult: &activityModels.Group{Name: "Freispiel"},
	}
	groupRepo.findByNameResult.ID = 72
	service := &fakeOperationsService{
		start: &scheduleSvc.StartInstanceResult{
			Instance:      startedInstance,
			ActiveGroupID: 342,
		},
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: service,
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}, ActivityGroupRepo: groupRepo, ActivityCategoryRepo: &fakeOperationActivityCategoryRepo{}}),
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
				person := &userModels.Person{}
				person.ID = 221
				return person, nil
			},
			GetStaffByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
				staff := &userModels.Staff{}
				staff.ID = 321
				return staff, nil
			},
		},
		SettingsService: &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "freispiel",
		"room_id": int64(70),
	})

	require.Equal(t, http.StatusCreated, rr.Code)
	require.NotNil(t, service.lastSpontaneousInput)
	require.NotNil(t, service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, int64(72), *service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, "freispiel", groupRepo.lastFindByName)
	assert.Nil(t, groupRepo.createdGroup, "existing activity should be reused, not recreated")
}

func TestOperationsCreateAndStartSpontaneousCreatesActivityForNewName(t *testing.T) {
	createdInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusPlanned}
	createdInstance.ID = 243
	startedInstance := &schedule.ActivityInstance{Status: schedule.InstanceStatusActive}
	startedInstance.ID = 243
	instanceSvc := &mockInstanceService{createRes: createdInstance}
	categoryRepo := &fakeOperationActivityCategoryRepo{}
	groupRepo := &fakeOperationActivityGroupRepo{createdID: 73}
	service := &fakeOperationsService{
		start: &scheduleSvc.StartInstanceResult{
			Instance:      startedInstance,
			ActiveGroupID: 343,
		},
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: service,
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}, ActivityGroupRepo: groupRepo, ActivityCategoryRepo: categoryRepo}),
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
				person := &userModels.Person{}
				person.ID = 222
				return person, nil
			},
			GetStaffByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
				staff := &userModels.Staff{}
				staff.ID = 322
				return staff, nil
			},
		},
		SettingsService: &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Neue Werkstatt",
		"room_id": int64(70),
	})

	require.Equal(t, http.StatusCreated, rr.Code)
	require.NotNil(t, categoryRepo.createdCategory)
	assert.Equal(t, "Spontan", categoryRepo.createdCategory.Name)
	require.NotNil(t, groupRepo.createdGroup)
	assert.Equal(t, "Neue Werkstatt", groupRepo.createdGroup.Name)
	assert.Equal(t, int64(910), groupRepo.createdGroup.CategoryID)
	assert.Equal(t, int64(322), *groupRepo.createdGroup.CreatedBy)
	require.NotNil(t, service.lastSpontaneousInput)
	require.NotNil(t, service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, int64(73), *service.lastSpontaneousInput.ActivityGroupID)
}

func TestOperationsCreateAndStartSpontaneousRejectsArchivedCategory(t *testing.T) {
	archivedAt := time.Now()
	categoryRepo := &fakeOperationActivityCategoryRepo{
		findByNameResult: &activityModels.Category{
			Name:       "Spontan",
			ArchivedAt: &archivedAt,
		},
	}
	groupRepo := &fakeOperationActivityGroupRepo{}
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		OperationsService: service,
		TimetableData: operationTimetableData(scheduleSvc.TimetableDataDependencies{
			ActiveGroupRepo:      &fakeOperationActiveGroupRepo{},
			ActivityGroupRepo:    groupRepo,
			ActivityCategoryRepo: categoryRepo,
		}),
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(_ context.Context, _ int64) (*userModels.Person, error) {
				person := &userModels.Person{}
				person.ID = 223
				return person, nil
			},
			GetStaffByPersonIDFn: func(_ context.Context, _ int64) (*userModels.Staff, error) {
				staff := &userModels.Staff{}
				staff.ID = 323
				return staff, nil
			},
		},
		SettingsService: &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Neue Werkstatt",
		"room_id": int64(70),
	})

	require.Equal(t, http.StatusConflict, rr.Code)
	assert.Nil(t, categoryRepo.createdCategory, "an archived Spontan category must not be recreated")
	assert.Nil(t, groupRepo.createdGroup)
	assert.Nil(t, service.lastSpontaneousInput)
}

func TestServerSpontaneousActivityWindowUsesBerlinServerTime(t *testing.T) {
	window := serverSpontaneousActivityWindow(time.Date(2026, 5, 12, 7, 5, 44, 0, time.UTC))

	assert.Equal(t, "2026-05-12", window.date.Format(dateLayout))
	assert.Equal(t, "09:05", window.startTime.Format("15:04"))
	assert.Equal(t, "10:05", window.endTime.Format("15:04"))
}

func TestServerSpontaneousActivityWindowCapsLateWindowSameDay(t *testing.T) {
	window := serverSpontaneousActivityWindow(time.Date(2026, 5, 12, 21, 45, 0, 0, time.UTC))

	assert.Equal(t, "2026-05-12", window.date.Format(dateLayout))
	assert.Equal(t, "23:30", window.startTime.Format("15:04"))
	assert.Equal(t, "23:59", window.endTime.Format("15:04"))
}

func TestOperationsCreateAndStartSpontaneousRejectsStudents(t *testing.T) {
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: &fakeOperationsService{},
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"date":        "2026-05-11",
		"start_time":  "14:00",
		"end_time":    "15:00",
		"title":       "Freispiel",
		"room_id":     7,
		"student_ids": []int64{99},
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsCreateAndStartSpontaneousRejectsWeekend(t *testing.T) {
	const roomID int64 = 7
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		SettingsService:   &fakeOperationSettingsService{hasOverride: true, boolValue: true},
		Now: func() time.Time {
			return time.Date(2026, time.May, 9, 14, 0, 0, 0, timezone.Berlin)
		},
	})
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Freispiel",
		"room_id": roomID,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Nil(t, service.lastSpontaneousInput, "weekend starts must not reach the operations service")
}

func TestOperationsCreateAndStartSpontaneousRejectsOccupiedRoom(t *testing.T) {
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{hasRoomConflict: true}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"date":       "2026-05-11",
		"start_time": "14:00",
		"end_time":   "15:00",
		"title":      "Freispiel",
		"room_id":    7,
	})

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Nil(t, service.lastSpontaneousInput, "occupied rooms must be rejected before creating an instance")
}

func TestOperationsCreateAndStartSpontaneousRequiresSetting(t *testing.T) {
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   false,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"date":       "2026-05-11",
		"start_time": "14:00",
		"end_time":   "15:00",
		"title":      "Freispiel",
		"room_id":    7,
	})

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Nil(t, service.lastSpontaneousInput, "disabled web spontaneous activities must not create instances")
}

func TestOperationsCreateAndStartSpontaneousRejectsFixedScheduleCareConcept(t *testing.T) {
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
			stringValue: configModel.CareConceptFixedSchedule,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(router, http.MethodPost, "/spontaneous/start", map[string]any{
		"date":       "2026-05-11",
		"start_time": "14:00",
		"end_time":   "15:00",
		"title":      "Freispiel",
		"room_id":    7,
	})

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Nil(t, service.lastSpontaneousInput, "fixed schedule must not create spontaneous instances")
}

func TestOperationsCapabilities(t *testing.T) {
	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	router := operationRouter(http.MethodGet, "/capabilities", res.operationsCapabilities)

	rr := executeOperationRequest(router, http.MethodGet, "/capabilities", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"web_spontaneous_activities_enabled":true`)
}

func TestOperationsCapabilitiesDefaultsToEnabled(t *testing.T) {
	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: false,
			boolValue:   false,
		},
	})
	router := operationRouter(http.MethodGet, "/capabilities", res.operationsCapabilities)

	rr := executeOperationRequest(router, http.MethodGet, "/capabilities", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"web_spontaneous_activities_enabled":true`)
}

func TestOperationsCapabilitiesDisabledForFixedScheduleCareConcept(t *testing.T) {
	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(scheduleSvc.TimetableDataDependencies{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
			stringValue: configModel.CareConceptFixedSchedule,
		},
	})
	router := operationRouter(http.MethodGet, "/capabilities", res.operationsCapabilities)

	rr := executeOperationRequest(router, http.MethodGet, "/capabilities", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"web_spontaneous_activities_enabled":false`)
}

func TestOperationsRosterByActiveGroup(t *testing.T) {
	service := &fakeOperationsService{
		roster: &scheduleSvc.OperationRoster{Instance: scheduleSvc.OperationRosterInstance{ID: 240}},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodGet, "/active-groups/{id}/roster", res.operationsRosterByActiveGroup)

	rr := executeOperationRequest(router, http.MethodGet, "/active-groups/340/roster", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(340), service.lastActiveGroupID)

	nilRes := NewResource(Dependencies{})
	nilRouter := operationRouter(http.MethodGet, "/active-groups/{id}/roster", nilRes.operationsRosterByActiveGroup)
	rr = executeOperationRequest(nilRouter, http.MethodGet, "/active-groups/340/roster", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/active-groups/nope/roster", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsStudentEndpoints(t *testing.T) {
	service := &fakeOperationsService{
		roster: &scheduleSvc.OperationRoster{Instance: scheduleSvc.OperationRosterInstance{ID: 250}},
	}
	res := NewResource(Dependencies{OperationsService: service})

	checkInRouter := operationRouter(http.MethodPost, "/instances/{id}/students/{student_id}/check-in", res.operationsCheckInStudent)
	rr := executeOperationRequest(checkInRouter, http.MethodPost, "/instances/250/students/350/check-in", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(250), service.lastInstanceID)
	assert.Equal(t, int64(350), service.lastStudentID)

	checkOutRouter := operationRouter(http.MethodPost, "/instances/{id}/students/{student_id}/check-out", res.operationsCheckOutStudent)
	rr = executeOperationRequest(checkOutRouter, http.MethodPost, "/instances/251/students/351/check-out", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(251), service.lastInstanceID)
	assert.Equal(t, int64(351), service.lastStudentID)

	rr = executeOperationRequest(checkInRouter, http.MethodPost, "/instances/bad/students/350/check-in", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	errorRouter := operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-in",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationConflict}}).operationsCheckInStudent,
	)
	rr = executeOperationRequest(errorRouter, http.MethodPost, "/instances/250/students/350/check-in", nil)
	assert.Equal(t, http.StatusConflict, rr.Code)

	errorRouter = operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-out",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationNotFound}}).operationsCheckOutStudent,
	)
	rr = executeOperationRequest(errorRouter, http.MethodPost, "/instances/250/students/350/check-out", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOperationsPatchAttendanceValidatesAndDelegates(t *testing.T) {
	status := schedule.AttendanceStatusAbsent
	service := &fakeOperationsService{
		patchRow: &scheduleSvc.OperationRosterRow{StudentID: 360, Status: schedule.AttendanceStatusAbsent},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)

	rr := executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{
		"status": status,
		"note":   "abgemeldet",
	})

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(260), service.lastInstanceID)
	assert.Equal(t, int64(360), service.lastStudentID)
	require.NotNil(t, service.lastPatch.Status)
	assert.Equal(t, status, *service.lastPatch.Status)
}

func TestOperationsPatchAttendanceRejectsBadRequests(t *testing.T) {
	nilService := NewResource(Dependencies{})
	nilRouter := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", nilService.operationsPatchAttendance)
	rr := executeOperationRequest(nilRouter, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"status": "present"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	res := NewResource(Dependencies{OperationsService: &fakeOperationsService{}})
	router := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)

	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", "{bad json")
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	res = NewResource(Dependencies{OperationsService: &fakeOperationsService{
		err: &scheduleSvc.TimetableAttendanceValidationError{
			Fields: []schedule.AttendancePatchFieldError{{Field: "substatus", Reason: "cannot be set when status is expected"}},
		},
	}})
	router = operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)
	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"substatus": "sick"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "substatus")

	res = NewResource(Dependencies{
		OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationForbidden},
	})
	router = operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)
	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"status": "absent"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestOperationsIDParsingAndErrorMapping(t *testing.T) {
	res := NewResource(Dependencies{OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationForbidden}})
	router := operationRouter(http.MethodGet, "/instances/{id}/roster", res.operationsRoster)
	rr := executeOperationRequest(router, http.MethodGet, "/instances/abc/roster", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/instances/260/roster", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	errorCases := []struct {
		err  error
		code int
	}{
		{scheduleSvc.ErrTimetableOperationNotFound, http.StatusNotFound},
		{scheduleSvc.ErrTimetableOperationConflict, http.StatusConflict},
		{scheduleSvc.ErrInvalidInstanceTransition, http.StatusConflict},
		{scheduleSvc.ErrInstanceNotFound, http.StatusNotFound},
		{activeSvc.ErrStudentAlreadyActive, http.StatusConflict},
		{activeSvc.ErrRoomConflict, http.StatusConflict},
		{activeSvc.ErrRoomCapacityExceeded, http.StatusConflict},
		{activeSvc.ErrActiveGroupAlreadyEnded, http.StatusConflict},
		{activeSvc.ErrStudentNotFound, http.StatusNotFound},
		{activeSvc.ErrVisitNotFound, http.StatusNotFound},
		{activeSvc.ErrInvalidData, http.StatusBadRequest},
		{errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range errorCases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			res := NewResource(Dependencies{OperationsService: &fakeOperationsService{err: tc.err}})
			router := operationRouter(http.MethodGet, "/instances/{id}/roster", res.operationsRoster)

			rr := executeOperationRequest(router, http.MethodGet, "/instances/260/roster", nil)

			assert.Equal(t, tc.code, rr.Code)
		})
	}
}

type fakeOperationsService struct {
	planned  []scheduleSvc.OperationPlannedInstance
	sessions []scheduleSvc.OperationActiveSession
	roster   *scheduleSvc.OperationRoster
	start    *scheduleSvc.StartInstanceResult
	complete *schedule.ActivityInstance
	patchRow *scheduleSvc.OperationRosterRow
	err      error

	lastAccountID        int64
	lastIsAdmin          bool
	lastDate             timezone.Date
	lastPlannedOptions   scheduleSvc.PlannedNowOptions
	lastInstanceID       int64
	lastActiveGroupID    int64
	lastStudentID        int64
	lastPatch            schedule.AttendanceFieldPatch
	lastSpontaneousInput *scheduleSvc.CreateInstanceInput
}

type fakeOperationActiveGroupRepo struct {
	activeModels.GroupRepository
	hasRoomConflict bool
	err             error
}

type fakeOperationRoomRepo struct {
	facilitiesModels.RoomRepository
	room *facilitiesModels.Room
	err  error
}

func operationTimetableData(deps scheduleSvc.TimetableDataDependencies) *scheduleSvc.TimetableDataService {
	if deps.ActiveGroupRepo == nil {
		deps.ActiveGroupRepo = &fakeOperationActiveGroupRepo{}
	}
	if deps.RoomRepo == nil {
		deps.RoomRepo = &fakeOperationRoomRepo{room: &facilitiesModels.Room{Name: "Lernraum"}}
	}
	return scheduleSvc.NewTimetableDataService(deps)
}

// Embedding stubs used to satisfy NewTimetableOperationsService's non-nil
// dependency check when building a REAL operations service for the spontaneous
// create+start rollback test. Only InstanceService, PersonService, Settings,
// ActiveGroupRepo, ActivityGroupRepo and RoomRepo are exercised by that flow;
// the rest exist solely to pass the constructor and panic if wrongly called.
type stubOpInstanceRepo struct {
	schedule.ActivityInstanceRepository
}
type stubOpInstanceStaffRepo struct {
	schedule.InstanceStaffRepository
}
type stubOpInstanceStudentRepo struct {
	schedule.InstanceStudentRepository
}
type stubOpSupervisorRepo struct {
	activeModels.GroupSupervisorRepository
}
type stubOpVisitRepo struct{ activeModels.VisitRepository }
type stubOpStudentRepo struct{ userModels.StudentRepository }
type stubOpEducationGroupRepo struct {
	educationModels.GroupRepository
}

type stubOpActiveService struct{}

func (stubOpActiveService) CreateVisit(context.Context, *activeModels.Visit) error { return nil }
func (stubOpActiveService) EndVisit(context.Context, int64) error                  { return nil }
func (stubOpActiveService) MoveStudentsToActiveGroupAuthorized(_ context.Context, studentIDs []int64, activeGroupID int64, _ activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
	return &activeSvc.StudentMoveResult{Moved: studentIDs, ActiveGroupID: &activeGroupID}, nil
}

type stubOpArrivalService struct{}

func (stubOpArrivalService) GetBulkEffectiveArrivalTimesForDate(context.Context, []int64, timezone.Date) (map[int64]*scheduleSvc.EffectiveArrivalTime, error) {
	return nil, nil
}

// stubOpCareDayService reports no care-plan verdicts, i.e. "unknown" for every
// child — the rosters and counts below therefore behave exactly as they did
// before the care-day derivation (#1747) existed.
type stubOpCareDayService struct{}

func (stubOpCareDayService) ResolveForDate(context.Context, []int64, timezone.Date) (map[int64]scheduleSvc.CareDayStatus, error) {
	return map[int64]scheduleSvc.CareDayStatus{}, nil
}

func (stubOpCareDayService) ResolveForRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64]map[timezone.Date]scheduleSvc.CareDayStatus, error) {
	return map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{}, nil
}

// newRealSpontaneousOpsService wires a production timetableOperationsService so
// the handler exercises the real CreateAndStartSpontaneous (Create + Start +
// MarkRollback), not a fake.
func newRealSpontaneousOpsService(db *bun.DB, instanceSvc scheduleSvc.InstanceService, personSvc scheduleSvc.OperationPersonService, settings scheduleSvc.OperationSettings) scheduleSvc.TimetableOperationsService {
	return scheduleSvc.NewTimetableOperationsService(scheduleSvc.TimetableOperationsDependencies{
		InstanceRepo:       stubOpInstanceRepo{},
		InstanceStaffRepo:  stubOpInstanceStaffRepo{},
		InstanceStudents:   stubOpInstanceStudentRepo{},
		InstanceService:    instanceSvc,
		ActiveGroupRepo:    &fakeOperationActiveGroupRepo{},
		ActivityGroupRepo:  &fakeOperationActivityGroupRepo{},
		ActiveService:      stubOpActiveService{},
		ArrivalService:     stubOpArrivalService{},
		CareDayService:     stubOpCareDayService{},
		SupervisorRepo:     stubOpSupervisorRepo{},
		VisitRepo:          stubOpVisitRepo{},
		StudentRepo:        stubOpStudentRepo{},
		EducationGroupRepo: stubOpEducationGroupRepo{},
		RoomRepo:           &fakeOperationRoomRepo{room: &facilitiesModels.Room{Name: "Lernraum"}},
		PersonService:      personSvc,
		Settings:           settings,
		DB:                 db,
	})
}

func (r *fakeOperationRoomRepo) FindByID(_ context.Context, _ interface{}) (*facilitiesModels.Room, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.room == nil {
		return nil, sql.ErrNoRows
	}
	return r.room, nil
}

type fakeOperationActivityGroupRepo struct {
	activityModels.GroupRepository
	findByNameResult *activityModels.Group
	findByNameErr    error
	createdGroup     *activityModels.Group
	createdID        int64
	lastFindByName   string
}

func (r *fakeOperationActivityGroupRepo) FindByName(_ context.Context, name string) (*activityModels.Group, error) {
	r.lastFindByName = name
	if r.findByNameErr != nil {
		return nil, r.findByNameErr
	}
	if r.findByNameResult != nil {
		return r.findByNameResult, nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeOperationActivityGroupRepo) Create(_ context.Context, group *activityModels.Group) error {
	r.createdGroup = group
	if r.createdID > 0 {
		group.ID = r.createdID
	}
	return nil
}

type fakeOperationActivityCategoryRepo struct {
	activityModels.CategoryRepository
	findByNameResult *activityModels.Category
	findByNameErr    error
	createdCategory  *activityModels.Category
}

func (r *fakeOperationActivityCategoryRepo) FindByName(_ context.Context, _ string) (*activityModels.Category, error) {
	return r.findByName()
}

func (r *fakeOperationActivityCategoryRepo) FindByNameIncludingArchivedForShare(_ context.Context, _ string) (*activityModels.Category, error) {
	return r.findByName()
}

func (r *fakeOperationActivityCategoryRepo) findByName() (*activityModels.Category, error) {
	if r.findByNameErr != nil {
		return nil, r.findByNameErr
	}
	if r.findByNameResult != nil {
		return r.findByNameResult, nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeOperationActivityCategoryRepo) Create(_ context.Context, category *activityModels.Category) error {
	r.createdCategory = category
	category.ID = 910
	return nil
}

type fakeOperationSettingsService struct {
	configSvc.SettingsService
	hasOverride bool
	boolValue   bool
	stringValue string
	err         error
}

func (s *fakeOperationSettingsService) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.hasOverride, nil
}

func (s *fakeOperationSettingsService) ResolveBool(_ context.Context, _ string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.boolValue, nil
}

func (s *fakeOperationSettingsService) ResolveString(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.stringValue, nil
}

func (r *fakeOperationActiveGroupRepo) CheckRoomConflict(_ context.Context, _ int64, _ int64) (bool, *activeModels.Group, error) {
	if r.err != nil {
		return false, nil, r.err
	}
	return r.hasRoomConflict, nil, nil
}

func (s *fakeOperationsService) PlannedNow(_ context.Context, accountID int64, isAdmin bool, date timezone.Date, _ time.Time, opts scheduleSvc.PlannedNowOptions) ([]scheduleSvc.OperationPlannedInstance, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastDate = date
	s.lastPlannedOptions = opts
	return s.planned, s.err
}

func (s *fakeOperationsService) ActiveSessions(_ context.Context, date timezone.Date) ([]scheduleSvc.OperationActiveSession, error) {
	s.lastDate = date
	return s.sessions, s.err
}

func (s *fakeOperationsService) Start(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleSvc.StartInstanceResult, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.start, s.err
}

func (s *fakeOperationsService) CreateAndStartSpontaneous(_ context.Context, accountID int64, isAdmin bool, in scheduleSvc.CreateInstanceInput) (*scheduleSvc.StartInstanceResult, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	inCopy := in
	s.lastSpontaneousInput = &inCopy
	if s.err != nil {
		return nil, s.err
	}
	if s.start != nil {
		s.lastInstanceID = s.start.Instance.ID
	}
	return s.start, nil
}

func (s *fakeOperationsService) Complete(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*schedule.ActivityInstance, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.complete, s.err
}

func (s *fakeOperationsService) Reopen(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleSvc.StartInstanceResult, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.start, s.err
}

func (s *fakeOperationsService) Roster(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.roster, s.err
}

func (s *fakeOperationsService) RosterByActiveGroup(_ context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastActiveGroupID = activeGroupID
	return s.roster, s.err
}

func (s *fakeOperationsService) CheckInStudent(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	return s.roster, s.err
}

func (s *fakeOperationsService) CheckOutStudent(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	return s.roster, s.err
}

func (s *fakeOperationsService) PatchAttendance(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch schedule.AttendanceFieldPatch) (*scheduleSvc.OperationRosterRow, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	s.lastPatch = patch
	return s.patchRow, s.err
}

func operationRouter(method, path string, handler http.HandlerFunc) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	switch method {
	case http.MethodGet:
		router.Get(path, handler)
	case http.MethodPost:
		router.Post(path, handler)
	case http.MethodPatch:
		router.Patch(path, handler)
	}
	return router
}

func executeOperationRequest(router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	switch v := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(v))
	default:
		raw, _ := json.Marshal(v)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	testutil.WithClaims(testutil.AdminTestClaims(120))(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestSpontaneousStartWorkdayWindow_RejectsWeekend(t *testing.T) {
	_, err := spontaneousStartWorkdayWindow(time.Date(2026, time.May, 9, 14, 0, 0, 0, timezone.Berlin))
	require.ErrorIs(t, err, errTimetableWeekend)
}

func lastPathSegment(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
