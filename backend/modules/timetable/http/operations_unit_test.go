package timetablehttp

import (
	"bytes"
	"context"
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
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	settingsContract "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationsPlannedNow(t *testing.T) {
	t.Parallel()

	service := &fakeOperationsService{
		planned: []timetable.OperationPlannedInstance{{ID: 220, Title: "Lernzeit"}},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)

	rr := executeOperationRequest(t, router, http.MethodGet, "/planned-now?date=2026-05-10", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(120), service.lastAccountID)
	assert.True(t, service.lastIsAdmin)
	assert.Equal(t, "2026-05-10", service.lastDate.Format(dateLayout))
	assert.Contains(t, rr.Body.String(), `"instances"`)

	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?horizon_minutes=480&limit=5&include_roster=true", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 480, service.lastPlannedOptions.HorizonMinutes)
	assert.Equal(t, 5, service.lastPlannedOptions.Limit)
	assert.True(t, service.lastPlannedOptions.IncludeRoster)

	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?scope=past", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, timetable.PlannedNowScopePast, service.lastPlannedOptions.Scope)
}

func TestOperationsRosterRoutesRedactPickupTimesWithoutStudentRead(t *testing.T) {
	t.Parallel()

	pickupTime := "15:00"
	resource := NewResource(Dependencies{OperationsService: &fakeOperationsService{
		roster: &timetable.OperationRoster{
			Rows:              []timetable.OperationRosterRow{{StudentID: 350, PickupTime: &pickupTime}},
			PickupTimesLoaded: true,
		},
	}})

	for _, tc := range []struct {
		route   string
		path    string
		handler http.HandlerFunc
	}{
		{"/instances/{id}/roster", "/instances/230/roster", resource.operationsRoster},
		{"/active-groups/{id}/roster", "/active-groups/340/roster", resource.operationsRosterByActiveGroup},
	} {
		t.Run(tc.path, func(t *testing.T) {
			router := operationRouter(http.MethodGet, tc.route, tc.handler)
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			testutil.WithClaims(t, jwt.AppClaims{ID: 120})(request)
			testutil.WithPermissions(permissions.SchedulesRead)(request)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), pickupTime)
			assert.Contains(t, response.Body.String(), `"pickup_times_loaded":false`)
			assert.Contains(t, response.Body.String(), `"pickup_times_redacted":true`)
		})
	}
}

func TestOperationsPlannedNowAllowsRosterFreeScheduleRead(t *testing.T) {
	t.Parallel()

	pickupTime := "15:00"
	service := &fakeOperationsService{planned: []timetable.OperationPlannedInstance{{
		ID:                220,
		RosterPreview:     []timetable.OperationRosterRow{{StudentID: 350, PickupTime: &pickupTime}},
		PickupTimesLoaded: true,
	}}}
	router := operationRouter(http.MethodGet, "/planned-now", NewResource(Dependencies{OperationsService: service}).operationsPlannedNow)
	req := httptest.NewRequest(http.MethodGet, "/planned-now", nil)
	testutil.WithClaims(t, jwt.AppClaims{ID: 120})(req)
	testutil.WithPermissions(permissions.SchedulesRead)(req)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, req)

	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.False(t, service.lastPlannedOptions.IncludeRoster)

	request := httptest.NewRequest(http.MethodGet, "/planned-now?include_roster=true", nil)
	testutil.WithClaims(t, jwt.AppClaims{ID: 120})(request)
	testutil.WithPermissions(permissions.SchedulesRead)(request)
	response = httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.NotContains(t, response.Body.String(), pickupTime)
	assert.Contains(t, response.Body.String(), `"pickup_times_loaded":false`)
	assert.Contains(t, response.Body.String(), `"pickup_times_redacted":true`)
}

func TestOperationsMutationResponsesRedactPickupTimesWithoutStudentRead(t *testing.T) {
	t.Parallel()

	pickupTime := "15:00"
	service := &fakeOperationsService{
		roster:   &timetable.OperationRoster{Rows: []timetable.OperationRosterRow{{StudentID: 350, PickupTime: &pickupTime}}, PickupTimesLoaded: true},
		patchRow: &timetable.OperationRosterRow{StudentID: 350, PickupTime: &pickupTime},
	}
	resource := NewResource(Dependencies{OperationsService: service})
	cases := []struct {
		method  string
		path    string
		body    any
		handler http.HandlerFunc
	}{
		{http.MethodPost, "/instances/250/students/350/check-in", nil, resource.operationsCheckInStudent},
		{http.MethodPost, "/instances/250/students/350/check-out", nil, resource.operationsCheckOutStudent},
		{http.MethodPatch, "/instances/250/students/350/attendance", map[string]any{"status": "absent"}, resource.operationsPatchAttendance},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			router := operationRouter(tc.method, "/instances/{id}/students/{student_id}/"+lastPathSegment(tc.path), tc.handler)
			body := bytes.NewReader(nil)
			if tc.body != nil {
				raw, err := json.Marshal(tc.body)
				require.NoError(t, err)
				body = bytes.NewReader(raw)
			}
			request := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != nil {
				request.Header.Set("Content-Type", "application/json")
			}
			testutil.WithClaims(t, jwt.AppClaims{ID: 120})(request)
			testutil.WithPermissions(permissions.SchedulesRead)(request)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.NotContains(t, response.Body.String(), pickupTime)
			if tc.method != http.MethodPatch {
				assert.Contains(t, response.Body.String(), `"pickup_times_loaded":false`)
				assert.Contains(t, response.Body.String(), `"pickup_times_redacted":true`)
			}
		})
	}
}

func testWorkdayNow() time.Time {
	return time.Date(2026, time.May, 11, 14, 0, 0, 0, calendar.Berlin)
}

func TestOperationsPlannedNowValidationAndWiring(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{})
	router := operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)
	rr := executeOperationRequest(t, router, http.MethodGet, "/planned-now", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	res = NewResource(Dependencies{OperationsService: &fakeOperationsService{}})
	router = operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)
	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?date=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?horizon_minutes=-1", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?include_roster=maybe", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?scope=future", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodGet, "/planned-now?scope=past&date=2000-01-01", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsInstanceEndpoints(t *testing.T) {
	t.Parallel()

	service := &fakeOperationsService{
		roster: &timetable.OperationRoster{Instance: timetable.OperationRosterInstance{ID: 230}},
		start: &timetable.StartedOperation{
			InstanceID:    231,
			Status:        timetable.InstanceStatusActive,
			ActiveGroupID: 330,
		},
		complete: &timetable.ScheduledInstance{ID: 232, Status: timetable.InstanceStatusCompleted},
	}
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

			rr := executeOperationRequest(t, router, tc.method, tc.path, tc.body)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
	assert.Equal(t, int64(232), service.lastInstanceID)
}

func TestOperationsReopenEffectiveAdminScope(t *testing.T) {
	t.Parallel()

	service := &fakeOperationsService{
		start: &timetable.StartedOperation{
			InstanceID:    231,
			Status:        timetable.InstanceStatusActive,
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
			testutil.WithClaims(t, jwt.AppClaims{ID: 120, IsAdmin: tc.isAdmin, TenantID: testpkg.Tenant(t)})(req)
			testutil.WithPermissions(tc.permissions...)(req)
			attachTestPrincipal(t, req, jwt.AppClaims{ID: 120, IsAdmin: tc.isAdmin, TenantID: testpkg.Tenant(t)}, tc.permissions)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
			assert.Equal(t, int64(120), service.lastAccountID)
			assert.Equal(t, tc.wantAdmin, service.lastIsAdmin)
		})
	}
}

func attachTestPrincipal(t *testing.T, req *http.Request, claims jwt.AppClaims, granted []string) {
	t.Helper()
	principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, Scope: claims.Scope,
		Roles: claims.Roles, Permissions: granted, Admin: claims.IsAdmin,
	})
	require.NoError(t, err)
	*req = *req.WithContext(permissions.WithPrincipal(req.Context(), principal))
}

func TestOperationsCreateAndStartSpontaneous(t *testing.T) {
	t.Parallel()

	createdInstance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusPlanned}
	createdInstance.ID = 241
	instanceSvc := &mockInstanceService{
		createRes: createdInstance,
	}
	service := &fakeOperationsService{
		start: &timetable.StartedOperation{
			InstanceID:    241,
			Status:        timetable.InstanceStatusActive,
			ActiveGroupID: 341,
		},
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: service,
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		People: &fakePeople{
			AccountPersonIDFn: func(_ context.Context, accountID int64) (int64, error) {
				assert.Equal(t, int64(120), accountID)
				return 220, nil
			},
			PersonStaffIDFn: func(_ context.Context, personID int64) (int64, error) {
				assert.Equal(t, int64(220), personID)
				return 320, nil
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

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
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
	assert.Equal(t, []int64{321, 320}, service.lastSpontaneousInput.StaffIDs)
	assert.Equal(t, calendar.NewDate(2026, 5, 11), service.lastSpontaneousInput.Date)
	assert.Equal(t, "14:00", service.lastSpontaneousInput.StartTime.Format("15:04"))
	assert.Equal(t, "15:00", service.lastSpontaneousInput.EndTime.Format("15:04"))
	assert.Equal(t, int64(241), service.lastInstanceID)
	assert.Contains(t, rr.Body.String(), `"active_group_id":341`)
}

func TestOperationsCreateAndStartSpontaneousRollsBackNon5xxFailures(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	probe := testpkg.NewTransactionProbe(t, db)

	createdInstance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusPlanned}
	createdInstance.ID = 244
	// Start fails non-5xx: the real service's Start delegates to InstanceService.Start,
	// which returns an invalid transition (→ 409). The Create write made just before
	// must roll back.
	instanceSvc := &rollbackProbeInstanceService{
		mockInstanceService: &mockInstanceService{startErr: timetable.ErrInvalidInstanceTransition},
		create: func(ctx context.Context, _ timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error) {
			require.NoError(t, probe.Write(ctx))
			require.True(t, probe.Written(), "probe write must happen inside the tenant transaction")
			return createdInstance, nil
		},
	}
	// The caller is an admin covered by the school-wide overview (#2380), so
	// the operation is not gated on a per-instance staff assignment.
	settings := &fakeOperationSettingsService{
		hasOverride: true,
		boolValue:   true,
		scope:       settingsContract.OverviewScopeAdmins,
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: newRealSpontaneousOpsService(t, instanceSvc, userstest.StaffAccount(224, 324), settings),
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		People:            staffAccountPeople(224, 324),
		SettingsService:   settings,
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

	body, err := json.Marshal(map[string]any{
		"title":             "Freispiel",
		"room_id":           int64(70),
		"activity_group_id": int64(71),
	})
	require.NoError(t, err)
	execute := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/spontaneous/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		testutil.WithClaims(t, testutil.AdminTestClaims(120))(req)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr
	}

	rr := execute()

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.False(t, probe.Committed(t),
		"a non-5xx start failure must roll back writes made before Start")

	createProbe := testpkg.NewTransactionProbe(t, db)
	// Create now fails non-5xx before Start is reached; the wrapping in
	// SpontaneousCreateError keeps the create-specific 400 mapping, and the
	// activity-resolution write must roll back.
	instanceSvc.create = func(ctx context.Context, _ timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error) {
		require.NoError(t, createProbe.Write(ctx))
		return nil, fmt.Errorf("%w: invalid staff_ids", timetable.ErrInvalidInstanceReference)
	}

	rr = execute()

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	require.True(t, createProbe.Written())
	assert.False(t, createProbe.Committed(t),
		"a non-5xx create failure must roll back activity resolution writes")
}

type rollbackProbeInstanceService struct {
	*mockInstanceService
	create func(context.Context, timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error)
}

func (s *rollbackProbeInstanceService) CreateInstance(ctx context.Context, req timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error) {
	return s.create(ctx, req)
}

func TestOperationsCreateAndStartSpontaneousReusesActivityByName(t *testing.T) {
	t.Parallel()

	createdInstance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusPlanned}
	createdInstance.ID = 242
	instanceSvc := &mockInstanceService{createRes: createdInstance}
	// The owner resolves "freispiel" to the existing "Freispiel" activity.
	activity := &fakeSpontaneousActivity{resolvedID: 72}
	service := &fakeOperationsService{
		start: &timetable.StartedOperation{
			InstanceID:    242,
			Status:        timetable.InstanceStatusActive,
			ActiveGroupID: 342,
		},
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: service,
		TimetableData:     activity.timetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		People:            staffAccountPeople(221, 321),
		SettingsService:   &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "freispiel",
		"room_id": int64(70),
	})

	require.Equal(t, http.StatusCreated, rr.Code)
	require.NotNil(t, service.lastSpontaneousInput)
	require.NotNil(t, service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, int64(72), *service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, 1, activity.calls)
	assert.Equal(t, "freispiel", activity.lastTitle, "the activity is resolved by the requested title")
	assert.Nil(t, activity.lastRequestedID, "without an activity_group_id the owner resolves by name")
	assert.Equal(t, int64(321), activity.lastCreatedBy)
}

func TestOperationsCreateAndStartSpontaneousCreatesActivityForNewName(t *testing.T) {
	t.Parallel()

	createdInstance := &timetable.LifecycleInstance{Status: timetable.InstanceStatusPlanned}
	createdInstance.ID = 243
	instanceSvc := &mockInstanceService{createRes: createdInstance}
	// The owner creates the new activity (in its "Spontan" category) as 73.
	activity := &fakeSpontaneousActivity{resolvedID: 73}
	service := &fakeOperationsService{
		start: &timetable.StartedOperation{
			InstanceID:    243,
			Status:        timetable.InstanceStatusActive,
			ActiveGroupID: 343,
		},
	}
	res := NewResource(Dependencies{
		InstanceService:   instanceSvc,
		OperationsService: service,
		TimetableData:     activity.timetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		People:            staffAccountPeople(222, 322),
		SettingsService:   &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Neue Werkstatt",
		"room_id": int64(70),
	})

	require.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, 1, activity.calls)
	assert.Equal(t, "Neue Werkstatt", activity.lastTitle)
	assert.Nil(t, activity.lastRequestedID)
	assert.Equal(t, int64(322), activity.lastCreatedBy, "the new activity is attributed to the starting staff member")
	require.NotNil(t, service.lastSpontaneousInput)
	require.NotNil(t, service.lastSpontaneousInput.ActivityGroupID)
	assert.Equal(t, int64(73), *service.lastSpontaneousInput.ActivityGroupID)
}

func TestOperationsCreateAndStartSpontaneousRejectsArchivedCategory(t *testing.T) {
	t.Parallel()

	// The owner refuses to file a new activity under an archived "Spontan"
	// category.
	activity := &fakeSpontaneousActivity{err: timetable.ErrSpontaneousCategoryArchived}
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		OperationsService: service,
		TimetableData: activity.timetableData(operationDataDeps{
			ActiveGroupRepo: &fakeOperationActiveGroupRepo{},
		}),
		People:          staffAccountPeople(223, 323),
		SettingsService: &fakeOperationSettingsService{hasOverride: true, boolValue: true},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Neue Werkstatt",
		"room_id": int64(70),
	})

	require.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, 1, activity.calls)
	assert.Contains(t, rr.Body.String(), timetable.ErrSpontaneousCategoryArchived.Error())
	assert.Nil(t, service.lastSpontaneousInput)
}

func TestServerSpontaneousActivityWindowUsesBerlinServerTime(t *testing.T) {
	t.Parallel()

	window := serverSpontaneousActivityWindow(time.Date(2026, 5, 12, 7, 5, 44, 0, time.UTC))

	assert.Equal(t, "2026-05-12", window.date.Format(dateLayout))
	assert.Equal(t, "09:05", window.startTime.Format("15:04"))
	assert.Equal(t, "10:05", window.endTime.Format("15:04"))
}

func TestServerSpontaneousActivityWindowCapsLateWindowSameDay(t *testing.T) {
	t.Parallel()

	window := serverSpontaneousActivityWindow(time.Date(2026, 5, 12, 21, 45, 0, 0, time.UTC))

	assert.Equal(t, "2026-05-12", window.date.Format(dateLayout))
	assert.Equal(t, "23:30", window.startTime.Format("15:04"))
	assert.Equal(t, "23:59", window.endTime.Format("15:04"))
}

func TestOperationsCreateAndStartSpontaneousRejectsStudents(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: &fakeOperationsService{},
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
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
	t.Parallel()

	const roomID int64 = 7
	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		SettingsService:   &fakeOperationSettingsService{hasOverride: true, boolValue: true},
		Now: func() time.Time {
			return time.Date(2026, time.May, 9, 14, 0, 0, 0, calendar.Berlin)
		},
	})
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Freispiel",
		"room_id": roomID,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Nil(t, service.lastSpontaneousInput, "weekend starts must not reach the operations service")
}

func TestOperationsCreateAndStartSpontaneousRejectsOccupiedRoom(t *testing.T) {
	t.Parallel()

	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{hasRoomConflict: true}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
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
	t.Parallel()

	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   false,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
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
	t.Parallel()

	service := &fakeOperationsService{}
	res := NewResource(Dependencies{
		TimetableData:     operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		InstanceService:   &mockInstanceService{},
		OperationsService: service,
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
			stringValue: settingsContract.CareConceptFixedSchedule,
		},
	})
	res.Now = testWorkdayNow
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
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
	t.Parallel()

	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
		},
	})
	router := operationRouter(http.MethodGet, "/capabilities", res.operationsCapabilities)

	rr := executeOperationRequest(t, router, http.MethodGet, "/capabilities", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"web_spontaneous_activities_enabled":true`)
}

func TestOperationsCapabilitiesDefaultsToEnabled(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: false,
			boolValue:   false,
		},
	})
	router := operationRouter(http.MethodGet, "/capabilities", res.operationsCapabilities)

	rr := executeOperationRequest(t, router, http.MethodGet, "/capabilities", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"web_spontaneous_activities_enabled":true`)
}

func TestOperationsCapabilitiesDisabledForFixedScheduleCareConcept(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{
		TimetableData: operationTimetableData(operationDataDeps{ActiveGroupRepo: &fakeOperationActiveGroupRepo{}}),
		SettingsService: &fakeOperationSettingsService{
			hasOverride: true,
			boolValue:   true,
			stringValue: settingsContract.CareConceptFixedSchedule,
		},
	})
	router := operationRouter(http.MethodGet, "/capabilities", res.operationsCapabilities)

	rr := executeOperationRequest(t, router, http.MethodGet, "/capabilities", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"web_spontaneous_activities_enabled":false`)
}

func TestOperationsRosterByActiveGroup(t *testing.T) {
	t.Parallel()

	service := &fakeOperationsService{
		roster: &timetable.OperationRoster{Instance: timetable.OperationRosterInstance{ID: 240}},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodGet, "/active-groups/{id}/roster", res.operationsRosterByActiveGroup)

	rr := executeOperationRequest(t, router, http.MethodGet, "/active-groups/340/roster", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(340), service.lastActiveGroupID)

	nilRes := NewResource(Dependencies{})
	nilRouter := operationRouter(http.MethodGet, "/active-groups/{id}/roster", nilRes.operationsRosterByActiveGroup)
	rr = executeOperationRequest(t, nilRouter, http.MethodGet, "/active-groups/340/roster", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodGet, "/active-groups/nope/roster", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsStudentEndpoints(t *testing.T) {
	t.Parallel()

	service := &fakeOperationsService{
		roster: &timetable.OperationRoster{Instance: timetable.OperationRosterInstance{ID: 250}},
	}
	res := NewResource(Dependencies{OperationsService: service})

	checkInRouter := operationRouter(http.MethodPost, "/instances/{id}/students/{student_id}/check-in", res.operationsCheckInStudent)
	rr := executeOperationRequest(t, checkInRouter, http.MethodPost, "/instances/250/students/350/check-in", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(250), service.lastInstanceID)
	assert.Equal(t, int64(350), service.lastStudentID)

	checkOutRouter := operationRouter(http.MethodPost, "/instances/{id}/students/{student_id}/check-out", res.operationsCheckOutStudent)
	rr = executeOperationRequest(t, checkOutRouter, http.MethodPost, "/instances/251/students/351/check-out", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(251), service.lastInstanceID)
	assert.Equal(t, int64(351), service.lastStudentID)

	rr = executeOperationRequest(t, checkInRouter, http.MethodPost, "/instances/bad/students/350/check-in", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	errorRouter := operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-in",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: timetable.ErrTimetableOperationConflict}}).operationsCheckInStudent,
	)
	rr = executeOperationRequest(t, errorRouter, http.MethodPost, "/instances/250/students/350/check-in", nil)
	assert.Equal(t, http.StatusConflict, rr.Code)

	errorRouter = operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-out",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: timetable.ErrTimetableOperationNotFound}}).operationsCheckOutStudent,
	)
	rr = executeOperationRequest(t, errorRouter, http.MethodPost, "/instances/250/students/350/check-out", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOperationsPatchAttendanceValidatesAndDelegates(t *testing.T) {
	t.Parallel()

	status := timetable.SlotAttendanceAbsent
	service := &fakeOperationsService{
		patchRow: &timetable.OperationRosterRow{StudentID: 360, Status: timetable.SlotAttendanceAbsent},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)

	rr := executeOperationRequest(t, router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{
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
	t.Parallel()

	nilService := NewResource(Dependencies{})
	nilRouter := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", nilService.operationsPatchAttendance)
	rr := executeOperationRequest(t, nilRouter, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"status": "present"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	res := NewResource(Dependencies{OperationsService: &fakeOperationsService{}})
	router := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)

	rr = executeOperationRequest(t, router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodPatch, "/instances/260/students/360/attendance", "{bad json")
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	res = NewResource(Dependencies{OperationsService: &fakeOperationsService{
		err: &timetable.AttendanceValidationError{
			Fields: []timetable.AttendanceFieldError{{Field: "substatus", Reason: "cannot be set when status is expected"}},
		},
	}})
	router = operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)
	rr = executeOperationRequest(t, router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"substatus": "sick"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "substatus")

	res = NewResource(Dependencies{
		OperationsService: &fakeOperationsService{err: timetable.ErrTimetableOperationForbidden},
	})
	router = operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)
	rr = executeOperationRequest(t, router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"status": "absent"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestOperationsIDParsingAndErrorMapping(t *testing.T) {
	t.Parallel()

	res := NewResource(Dependencies{OperationsService: &fakeOperationsService{err: timetable.ErrTimetableOperationForbidden}})
	router := operationRouter(http.MethodGet, "/instances/{id}/roster", res.operationsRoster)
	rr := executeOperationRequest(t, router, http.MethodGet, "/instances/abc/roster", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(t, router, http.MethodGet, "/instances/260/roster", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	errorCases := []struct {
		err  error
		code int
	}{
		{timetable.ErrTimetableOperationNotFound, http.StatusNotFound},
		{timetable.ErrTimetableOperationConflict, http.StatusConflict},
		{timetable.ErrInvalidInstanceTransition, http.StatusConflict},
		{timetable.ErrInstanceNotFound, http.StatusNotFound},
		{studentpresence.ErrStudentAlreadyActive, http.StatusConflict},
		{studentpresence.ErrRoomConflict, http.StatusConflict},
		{studentpresence.ErrRoomCapacityExceeded, http.StatusConflict},
		{studentpresence.ErrGroupAlreadyEnded, http.StatusConflict},
		{studentpresence.ErrStudentNotFound, http.StatusNotFound},
		{studentpresence.ErrVisitNotFound, http.StatusNotFound},
		{studentpresence.ErrInvalidData, http.StatusBadRequest},
		{errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range errorCases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			res := NewResource(Dependencies{OperationsService: &fakeOperationsService{err: tc.err}})
			router := operationRouter(http.MethodGet, "/instances/{id}/roster", res.operationsRoster)

			rr := executeOperationRequest(t, router, http.MethodGet, "/instances/260/roster", nil)

			assert.Equal(t, tc.code, rr.Code)
		})
	}
}

type fakeOperationsService struct {
	planned  []timetable.OperationPlannedInstance
	sessions []timetable.OperationActiveSession
	roster   *timetable.OperationRoster
	start    *timetable.StartedOperation
	complete *timetable.ScheduledInstance
	patchRow *timetable.OperationRosterRow
	err      error

	lastAccountID        int64
	lastIsAdmin          bool
	lastDate             calendar.Date
	lastPlannedOptions   timetable.PlannedNowOptions
	lastInstanceID       int64
	lastActiveGroupID    int64
	lastStudentID        int64
	lastPatch            timetable.AttendancePatch
	lastSpontaneousInput *timetable.SpontaneousStart
}

type fakeOperationActiveGroupRepo struct {
	studentpresence.SessionRecords
	hasRoomConflict bool
	err             error
}

// fakeOperationRooms names the Facilities rooms for the planner reads: every
// room is called name, and without a name no room exists.
type fakeOperationRooms struct {
	timetableCompose.RoomNames
	name string
	err  error
}

// operationDataDeps names the fakes the spontaneous-start tests drive through
// the Timetable owner's real planner reads. Occupancy and room default to a
// free "Lernraum"; the activity resolution by name panics when reached (the
// tests that reach it fake it at the capability, see fakeSpontaneousActivity).
type operationDataDeps struct {
	ActiveGroupRepo studentpresence.SessionRecords
	Rooms           timetableCompose.RoomNames
}

func operationTimetableData(deps operationDataDeps) timetable.TimetableDataCapability {
	if deps.ActiveGroupRepo == nil {
		deps.ActiveGroupRepo = &fakeOperationActiveGroupRepo{}
	}
	if deps.Rooms == nil {
		deps.Rooms = fakeOperationRooms{name: "Lernraum"}
	}
	return unitTimetableData(unitDataDeps{
		Sessions: deps.ActiveGroupRepo,
		Rooms:    deps.Rooms,
	})
}

// fakeSpontaneousActivity answers the Timetable owner's spontaneous activity
// resolution (TimetableDataCapability.ResolveSpontaneousActivity) with the
// activity the owner would resolve or create, or with its refusal, and
// records what the handler asked for. The room checks keep running through
// the owner's real planner reads.
type fakeSpontaneousActivity struct {
	resolvedID int64
	err        error

	calls           int
	lastTitle       string
	lastRequestedID *int64
	lastCreatedBy   int64
}

func (f *fakeSpontaneousActivity) timetableData(deps operationDataDeps) timetable.TimetableDataCapability {
	return &fakeTimetableData{
		TimetableDataCapability:      operationTimetableData(deps),
		ResolveSpontaneousActivityFn: f.resolve,
	}
}

func (f *fakeSpontaneousActivity) resolve(_ context.Context, title string, requestedID *int64, createdBy int64) (*int64, error) {
	f.calls++
	f.lastTitle = title
	f.lastRequestedID = requestedID
	f.lastCreatedBy = createdBy
	if f.err != nil {
		return nil, f.err
	}
	id := f.resolvedID
	return &id, nil
}

// Embedding stubs used to satisfy the operational day's non-nil dependency
// check when building the REAL Timetable operations for the spontaneous
// create+start rollback test. Only the lifecycle, the people and the
// settings are exercised by that flow; the rest exist solely to pass the
// constructor and panic if wrongly called.
type (
	stubOpInstances struct {
		timetableCompose.OperationInstances
	}
	stubOpInstanceStaff struct {
		timetableCompose.OperationInstanceStaff
	}
	stubOpParticipants struct {
		timetableCompose.OperationParticipants
	}
	stubOpTemplates struct {
		timetableCompose.OperationTemplates
	}
	stubOpSessions struct {
		timetableCompose.OperationSessions
	}
	stubOpSupervisions struct {
		timetableCompose.OperationSupervisions
	}
	stubOpVisits struct {
		timetableCompose.OperationVisits
	}
	stubOpAttendance struct {
		timetableCompose.OperationAttendance
	}
	stubOpStudents struct {
		timetableCompose.OperationStudents
	}
	stubOpEducationGroups struct{}
	stubOpRooms           struct{ timetableCompose.RoomNames }
	stubOpCareDays        struct{ timetableCompose.CareDays }
	stubOpArrivals        struct{}
	stubOpPickups         struct{}
)

func (stubOpEducationGroups) EducationGroupNames(context.Context, []int64) (map[int64]string, error) {
	panic("unused")
}

func (stubOpArrivals) EffectiveArrivals(context.Context, []int64, calendar.Date) (map[int64]*timetableCompose.ExpectedArrival, error) {
	panic("unused")
}

func (stubOpPickups) EffectivePickups(context.Context, []int64, calendar.Date) (map[int64]*time.Time, error) {
	panic("unused")
}

// testOperationSettings resolves the operational policies from the fake
// settings service. The action scopes stay unset, so no action opens to the
// whole school.
type testOperationSettings struct {
	settings *fakeOperationSettingsService
}

// testActionScopeKey stands for every action scope setting.
const testActionScopeKey = "operations.attendance_edit_scope"

func (s testOperationSettings) ResolveString(ctx context.Context, key string) (string, error) {
	if key == testActionScopeKey {
		return "", nil
	}
	return s.settings.ResolveString(ctx, key)
}

func (testOperationSettings) StartLeadMinutes(context.Context) (int, error) { return 0, nil }

func (testOperationSettings) EnforcePlannedEnd(context.Context) (bool, error) { return false, nil }

func (testOperationSettings) ActionScopeKey(timetableCompose.ScopedAction) (string, error) {
	return testActionScopeKey, nil
}

func (s testOperationSettings) StudentAbsenceEditAllStaff(ctx context.Context) (bool, error) {
	scope, err := s.settings.ResolveString(ctx, settingsContract.KeyStudentAbsenceEditScope)
	return scope == settingsContract.StudentAbsenceEditScopeAllStaff, err
}

// testOperationLifecycle drives the instance lifecycle the way the owner
// binds it (modules/timetable/compose/instance_lifecycle_ports.go).
type testOperationLifecycle struct {
	instances timetable.InstanceLifecycleCapability
}

func (l testOperationLifecycle) CreateSpontaneous(ctx context.Context, in timetable.SpontaneousStart) (int64, error) {
	spontaneous := true
	instance, err := l.instances.CreateInstance(ctx, timetable.CreateInstanceInput{
		Date: in.Date, StartTime: in.StartTime, EndTime: in.EndTime, Title: in.Title,
		Description: in.Description, Notes: in.Notes, RoomID: in.RoomID, ActivityGroupID: in.ActivityGroupID,
		IsSpontaneous: &spontaneous, StaffIDs: in.StaffIDs, CreatedByStaffID: in.CreatedByStaffID,
	})
	if err != nil {
		return 0, err
	}
	return instance.ID, nil
}

func (l testOperationLifecycle) Start(ctx context.Context, instanceID, staffID int64, spontaneous bool) (*timetable.StartedOperation, error) {
	if spontaneous {
		ctx = timetable.WithSpontaneousStartWorkdayGuard(ctx)
	}
	result, err := l.instances.Start(ctx, instanceID, staffID)
	if err != nil {
		return nil, err
	}
	return &timetable.StartedOperation{InstanceID: result.Instance.ID, Status: result.Instance.Status, ActiveGroupID: result.ActiveGroupID}, nil
}

func (testOperationLifecycle) Complete(context.Context, int64, int64) (*timetable.ScheduledInstance, error) {
	panic("unused")
}

func (testOperationLifecycle) Reopen(context.Context, int64, int64, bool) (*timetable.StartedOperation, error) {
	panic("unused")
}

// newRealSpontaneousOpsService wires the Timetable owner's real operational
// day so the handler exercises the real CreateAndStartSpontaneous (Create +
// Start + MarkRollback), not a fake.
func newRealSpontaneousOpsService(t *testing.T, instanceSvc timetable.InstanceLifecycleCapability, personSvc *userstest.PersonServiceMock, settings *fakeOperationSettingsService) timetable.OperationCapability {
	t.Helper()
	operations, err := timetableCompose.NewOperations(timetableCompose.OperationDependencies{
		Instances:            stubOpInstances{},
		InstanceStaff:        stubOpInstanceStaff{},
		Participants:         stubOpParticipants{},
		Templates:            stubOpTemplates{},
		Sessions:             stubOpSessions{},
		Supervisions:         stubOpSupervisions{},
		Visits:               stubOpVisits{},
		Attendance:           stubOpAttendance{},
		Students:             stubOpStudents{},
		People:               personSvc,
		EducationGroups:      stubOpEducationGroups{},
		Rooms:                stubOpRooms{},
		Settings:             testOperationSettings{settings: settings},
		CareDays:             stubOpCareDays{},
		Arrivals:             stubOpArrivals{},
		Pickups:              stubOpPickups{},
		Lifecycle:            testOperationLifecycle{instances: instanceSvc},
		NormalizeSchoolClass: func(class string) string { return class },
	})
	require.NoError(t, err)
	return operations
}

func (r fakeOperationRooms) RoomName(context.Context, int64) (string, bool, error) {
	if r.err != nil {
		return "", false, r.err
	}
	if r.name == "" {
		return "", false, facilities.ErrRoomNotFound
	}
	return r.name, true, nil
}

type fakeOperationSettingsService struct {
	settingsContract.Resolver
	hasOverride bool
	boolValue   bool
	stringValue string
	// scope answers operations.operational_overview_scope only (#2380), so a
	// test can open the school-wide overview without changing what every
	// other string setting resolves to.
	scope string
	err   error
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

func (s *fakeOperationSettingsService) ResolveString(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if key == settingsContract.KeyOperationalOverviewScope {
		return s.scope, nil
	}
	return s.stringValue, nil
}

func (r *fakeOperationActiveGroupRepo) CheckRoomConflict(_ context.Context, _ int64, _ int64) (bool, *studentpresence.LiveGroup, error) {
	if r.err != nil {
		return false, nil, r.err
	}
	return r.hasRoomConflict, nil, nil
}

func (s *fakeOperationsService) PlannedNow(_ context.Context, accountID int64, isAdmin bool, date calendar.Date, _ time.Time, opts timetable.PlannedNowOptions) ([]timetable.OperationPlannedInstance, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastDate = date
	s.lastPlannedOptions = opts
	return s.planned, s.err
}

func (s *fakeOperationsService) ActiveSessions(_ context.Context, date calendar.Date) ([]timetable.OperationActiveSession, error) {
	s.lastDate = date
	return s.sessions, s.err
}

func (s *fakeOperationsService) Start(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.StartedOperation, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.start, s.err
}

func (s *fakeOperationsService) CreateAndStartSpontaneous(_ context.Context, accountID int64, isAdmin bool, in timetable.SpontaneousStart) (*timetable.StartedOperation, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	inCopy := in
	s.lastSpontaneousInput = &inCopy
	if s.err != nil {
		return nil, s.err
	}
	if s.start != nil {
		s.lastInstanceID = s.start.InstanceID
	}
	return s.start, nil
}

func (s *fakeOperationsService) Complete(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.ScheduledInstance, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.complete, s.err
}

func (s *fakeOperationsService) Reopen(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.StartedOperation, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.start, s.err
}

func (s *fakeOperationsService) Roster(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.roster, s.err
}

func (s *fakeOperationsService) RosterByActiveGroup(_ context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*timetable.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastActiveGroupID = activeGroupID
	return s.roster, s.err
}

func (s *fakeOperationsService) CheckInStudent(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	return s.roster, s.err
}

func (s *fakeOperationsService) CheckOutStudent(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*timetable.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	return s.roster, s.err
}

func (s *fakeOperationsService) PatchAttendance(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch timetable.AttendancePatch) (*timetable.OperationRosterRow, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	s.lastPatch = patch
	return s.patchRow, s.err
}

// EarliestPlannedBlockStartForClass exists only to satisfy the interface
// (#2970); no handler in this package calls it.
func (s *fakeOperationsService) EarliestPlannedBlockStartForClass(context.Context, string, calendar.Date) (string, error) {
	return "", s.err
}

// SessionBlocks serves the supervision projection (#3281); no handler in this
// package calls it.
func (s *fakeOperationsService) SessionBlocks(context.Context, int64, bool, calendar.Date, map[int64][]int64) ([]timetable.OperationSessionBlock, error) {
	return nil, s.err
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

func executeOperationRequest(tb testing.TB, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	tb.Helper()
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
	testutil.WithClaims(tb, testutil.AdminTestClaims(120))(req)
	testutil.WithPermissions(permissions.UsersRead)(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestSpontaneousStartWorkdayWindow_RejectsWeekend(t *testing.T) {
	t.Parallel()

	_, err := spontaneousStartWorkdayWindow(time.Date(2026, time.May, 9, 14, 0, 0, 0, calendar.Berlin))
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

// TestOperationsCheckInParticipantLimitWire pins that a Betreuungsplan
// check-in refused by the activity's participant limit (#3632) answers 409
// with the refusal's code and numbers, not a server error.
func TestOperationsCheckInParticipantLimitWire(t *testing.T) {
	t.Parallel()
	limitErr := &studentpresence.OperationError{Op: "CreateVisit", Err: &studentpresence.ActivityParticipantLimitError{
		ActivityID: 7, ActivityName: "Fußball", CurrentOccupancy: 45, MaxParticipants: 45, Incoming: 1,
	}}
	router := operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-in",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: limitErr}}).operationsCheckInStudent,
	)

	rr := executeOperationRequest(t, router, http.MethodPost, "/instances/250/students/350/check-in", nil)

	require.Equal(t, http.StatusConflict, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, studentpresence.ActivityParticipantLimitCode, body["code"])
	details, ok := body["details"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(45), details["max_participants"])
	assert.Equal(t, float64(1), details["incoming_students"])
}
