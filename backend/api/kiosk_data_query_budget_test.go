//go:build testhooks

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	iotcompose "github.com/moto-nrw/project-phoenix/api/iot/compose"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Scope: existing root compositions + real API-key/PIN
// middleware + existing settings/identity memos + tenant transactions/RLS.
// This is not the outer Serve host/router, rate limit or network listener.
// Independent wire expectations avoid importing the process-owned facade.
type kioskBudgetActivity struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	IsOccupied bool   `json:"is_occupied"`
}

type kioskBudgetRoom struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Building   string  `json:"building,omitempty"`
	Floor      *int    `json:"floor,omitempty"`
	Capacity   *int    `json:"capacity,omitempty"`
	Category   *string `json:"category,omitempty"`
	Color      *string `json:"color,omitempty"`
	IsOccupied bool    `json:"is_occupied"`
	IsOpenRoom bool    `json:"is_open_room"`
	IsSchulhof bool    `json:"is_schulhof"`
}

type kioskBudgetTeacher struct {
	StaffID     int64  `json:"staff_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// Dependencies remain local values rather than a root-owned service graph.
func newKioskDataBudgetFixture(t *testing.T) (*bun.DB, http.Handler, activityContract.ActivityService, DeviceTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	data, _, activity, _, devices, _, _, err := NewIoTDataTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	_, settings := setupIoTSettingsModule(t)
	feedback, err := NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	identity, err := NewRFIDTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	schools, err := NewDeviceSchools(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	auth := testutil.NewDeviceAuthenticators(devices.IoT.Fleet(), schools, settings)
	resource := iotcompose.NewResource(iotcompose.ServiceDependencies{
		Runtime: NewIoTHTTPRuntime(), ErrorReports: NewIoTErrorReporter(nil),
		Administration: NewIoTAdministration(devicefleet.NewAdministration(devices.IoT.Fleet())),
		Directory:      data.Directory, Rooms: data.RoomAvailability, Configuration: data.Configuration,
		FeedbackService: feedback.Feedback, FeedbackStudents: identity.FeedbackStudents,
		FeedbackResponseObserver: func(int, string) {},
		// Existing route-composition placeholders. These write routes are NOT
		// exercised; all measured data/room/teacher/auth collaborators are real.
		DeviceScan: routerDeviceScan{}, StaffClock: routerStaffClock{},
		DeviceAuthenticator: auth.Device(), DeviceOnlyAuthenticator: auth.DeviceOnly(),
	})
	root := chi.NewRouter()
	root.Use(RequestSettingsCacheMiddleware)
	root.Use(RequestIdentityCacheMiddleware)
	root.Mount("/api/iot", resource.Router())
	return db, testpkg.TenantRuntimeMiddleware(t, db)(root), activity, devices
}

// Every request gets a fresh REAL device before capture. Its first successful
// authentication must perform exactly one synchronous last_seen UPDATE; the
// existing BEFORE UPDATE trigger also stamps updated_at. Reusing
// a device here would warm the existing debouncer and schedule a delayed write.
// No timers/cache overrides, sleeps, new caches or fake auth are introduced.
func requestKioskData(t *testing.T, db *bun.DB, handler http.Handler, devices DeviceTestModule, path string, withPIN bool) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	device := testpkg.CreateTestDevice(t, db, "kiosk-data-budget")
	require.NotNil(t, device.APIKey)
	before, err := devices.IoT.Fleet().FindDeviceByAPIKey(testpkg.Ctx(t), *device.APIKey)
	require.NoError(t, err)
	require.Nil(t, before.LastSeen)
	counter := testpkg.CaptureQueriesForContext(t, db)
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+*device.APIKey)
	if withPIN {
		// Existing test fixture PIN, not a deployed credential. The real
		// Settings resolver supplies it; no env fallback or stub is used.
		request.Header.Set("X-Staff-PIN", "1234")
	}
	request = request.WithContext(counter.Context(request.Context()))
	// PostgreSQL and Go may run in different clock domains. These two
	// infrastructure-only observations use the unmarked fixture context,
	// never counter.Context, and do not enter the request SQL budget.
	var databaseStarted, databaseFinished time.Time
	require.NoError(t, db.NewRaw("SELECT clock_timestamp()").Scan(testpkg.Ctx(t), &databaseStarted))
	counter.Reset()
	started := time.Now().Truncate(time.Microsecond)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	finished := time.Now().Add(time.Microsecond)
	require.NoError(t, db.NewRaw("SELECT clock_timestamp()").Scan(testpkg.Ctx(t), &databaseFinished))
	queries := counter.Queries()
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Header().Get("Content-Type"), "application/json")
	for _, operation := range []string{"INSERT", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation), "GET may write only the authenticated device heartbeat")
	}
	updates := counter.Operation("UPDATE")
	require.Len(t, updates, 1, "fresh successful auth writes last_seen once; domain GET must add no DML")
	heartbeat := strings.ToLower(strings.Join(strings.Fields(updates[0]), " "))
	require.Contains(t, heartbeat, "update iot.devices")
	// UpdateDeviceLastSeen names only last_seen. updated_at is trigger-owned,
	// not a second explicitly writable column.
	_, setAndWhere, hasSet := strings.Cut(heartbeat, " set ")
	require.True(t, hasSet)
	setClause, _, hasWhere := strings.Cut(setAndWhere, " where ")
	require.True(t, hasWhere)
	require.True(t, strings.HasPrefix(setClause, `"last_seen" = `))
	require.NotContains(t, setClause, ",", "heartbeat must explicitly assign only last_seen")
	require.Contains(t, heartbeat, fmt.Sprintf(`"device".id = %d`, device.ID))
	require.EqualValues(t, 1, counter.WriteRows())
	// Owner reads are deliberately outside the counter's request context.
	after, err := devices.IoT.Fleet().FindDeviceByAPIKey(testpkg.Ctx(t), *device.APIKey)
	require.NoError(t, err)
	require.NotNil(t, after.LastSeen)
	require.False(t, after.LastSeen.Before(started))
	require.False(t, after.LastSeen.After(finished))
	// 001003009_iot_devices installs BEFORE UPDATE update_modified_column;
	// 000001_core_functions assigns NEW.updated_at = now(). RecordLastSeen
	// masks any ambient transaction, so its own tenant write unit is opened
	// and committed during this ServeHTTP call. NOW() is that database unit's
	// timestamp, not the application's LastSeen time.Now(). Bound UpdatedAt
	// only by database observations, without rounding or time tolerance;
	// LastSeen retains its independent application-clock interval above.
	require.False(t, after.UpdatedAt.IsZero())
	require.True(t, after.UpdatedAt.After(before.UpdatedAt), "heartbeat trigger must advance updated_at")
	require.False(t, databaseFinished.Before(databaseStarted), "database clock must not run backwards during the request")
	require.False(t, after.UpdatedAt.Before(databaseStarted), "trigger timestamp must belong to this request's tenant write unit")
	require.False(t, after.UpdatedAt.After(databaseFinished), "heartbeat transaction must finish within the database-observed request window")
	before.LastSeen = after.LastSeen
	before.UpdatedAt = after.UpdatedAt
	require.Equal(t, before, after, "only bounded last_seen and trigger-owned updated_at may change")
	require.NotEmpty(t, queries)
	return response, queries
}

func decodeKioskDataRows[T any](t *testing.T, response *httptest.ResponseRecorder, message string) []T {
	t.Helper()
	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    []T    `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body.Bytes()))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&body))
	require.Equal(t, "success", body.Status)
	require.Equal(t, message, body.Message)
	require.NotNil(t, body.Data, "positive roster must be an array, not null")
	canonical, err := json.Marshal(body)
	require.NoError(t, err)
	require.JSONEq(t, string(canonical), response.Body.String(), "exact wire DTO field presence/omitempty shape; array order is not normalized")
	return body.Data
}

func TestKioskDataQueryBudgetActivities(t *testing.T) {
	t.Parallel()
	db, handler, activityService, devices := newKioskDataBudgetFixture(t)
	foreign := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreign)
	foreignActivity := testpkg.CreateTestActivityGroupForTenant(t, db, foreign, "foreign-kiosk-activity")
	want := map[int64]kioskBudgetActivity{}
	categoryIDs := map[int64]bool{}
	add := func(first, end int) {
		t.Helper()
		for i := first; i < end; i++ {
			// A single GetGroup intentionally returns no Category relation. Build
			// a real, distinct category and associate the group explicitly through
			// the existing owner capability; keep the independent fixture name
			// as the wire expectation instead of reading it from the endpoint.
			category := testpkg.CreateTestActivityCategory(t, db, fmt.Sprintf("kiosk-category-%02d", i))
			require.NotContains(t, categoryIDs, category.ID)
			categoryIDs[category.ID] = true
			creator := testpkg.CreateTestStaff(t, db, "Activity", fmt.Sprintf("Creator%02d", i))
			var activity *activityContract.Group
			// Category validation is part of the same real tenant transaction as
			// the owner command, before the request counter starts.
			err := testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context) error {
				var createErr error
				activity, createErr = activityService.CreateGroup(ctx, &activityContract.Group{
					Name: fmt.Sprintf("kiosk-activity-%02d", 99-i), MaxParticipants: 20, IsOpen: true,
					CategoryID: category.ID, CreatedBy: &creator.ID,
				}, []int64{}, []*activityContract.Schedule{})
				return createErr
			})
			require.NoError(t, err)
			require.NotNil(t, activity)
			require.Equal(t, category.ID, activity.CategoryID)
			require.NotNil(t, activity.Category)
			require.Equal(t, category.ID, activity.Category.ID)
			require.Equal(t, category.Name, activity.Category.Name)
			require.NotEmpty(t, category.Name)
			occupied := i%2 == 0
			if occupied {
				room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("activity-room-%02d", i))
				testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			}
			want[activity.ID] = kioskBudgetActivity{ID: activity.ID, Name: activity.Name, Category: category.Name, IsOccupied: occupied}
		}
	}
	run := func() []string {
		t.Helper()
		// The production GroupFilter{} query has NO ORDER BY. Do not invent
		// name/ID order or sort the measured wire. Capture an original control
		// at this N, verify exact fixture records, then compare the full ordered
		// array. This pins this fixture's observed sequence, not a global SQL
		// ordering guarantee. A cross-revision order claim still needs L2.
		control, _ := requestKioskData(t, db, handler, devices, "/api/iot/activities", true)
		order := decodeKioskDataRows[kioskBudgetActivity](t, control, "Activities fetched successfully")
		response, queries := requestKioskData(t, db, handler, devices, "/api/iot/activities", true)
		rows := decodeKioskDataRows[kioskBudgetActivity](t, response, "Activities fetched successfully")
		require.Equal(t, order, rows, "do not normalize response-array order")
		require.Len(t, rows, len(want))
		got := map[int64]kioskBudgetActivity{}
		for _, row := range rows {
			require.NotContains(t, got, row.ID)
			require.NotEqual(t, foreignActivity.ID, row.ID)
			got[row.ID] = row
		}
		require.Equal(t, want, got, "all exact names/categories/occupancy must be returned; no tenant leak")
		return queries
	}
	add(0, 3)
	small := run()
	querybudgets.AssertQueryBudget(t, "api.iot.activities.list", small)
	add(3, 8)
	large := run()
	require.Len(t, large, len(small), "3→8 distinct activities and occupancy relations must stay flat")
	querybudgets.AssertQueryBudget(t, "api.iot.activities.list", large)
}

func TestKioskDataQueryBudgetAvailableRooms(t *testing.T) {
	t.Parallel()
	db, handler, _, devices := newKioskDataBudgetFixture(t)
	foreign := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreign)
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreign, "foreign-kiosk-room")
	want := []kioskBudgetRoom{}
	add := func(first, end int) {
		t.Helper()
		for i := first; i < end; i++ {
			room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("kiosk-room-%02d", 99-i))
			activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("room-activity-%02d", i))
			session := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			staff := testpkg.CreateTestStaff(t, db, "Room", fmt.Sprintf("Supervisor%02d", i))
			testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
			student := testpkg.CreateTestStudent(t, db, "Room", fmt.Sprintf("Visitor%02d", i), "3a")
			testpkg.CreateTestVisit(t, db, student.ID, session.ID, session.StartTime, nil)
			want = append(want, kioskBudgetRoom{ID: room.ID, Name: room.Name, Building: room.Building,
				Floor: room.Floor, Capacity: room.Capacity, Category: room.Category, Color: room.Color,
				IsOccupied: true, IsOpenRoom: room.IsOpenRoom, IsSchulhof: false})
		}
	}
	run := func() []string {
		t.Helper()
		response, queries := requestKioskData(t, db, handler, devices, "/api/iot/rooms/available?capacity=10", true)
		rows := decodeKioskDataRows[kioskBudgetRoom](t, response, "Available rooms retrieved successfully")
		expected := slices.Clone(want)
		slices.SortFunc(expected, func(a, b kioskBudgetRoom) int {
			if byName := strings.Compare(a.Name, b.Name); byName != 0 {
				return byName
			}
			if a.ID < b.ID {
				return -1
			}
			if a.ID > b.ID {
				return 1
			}
			return 0
		})
		require.Equal(t, expected, rows, "Facilities orders by name ASC,id ASC; preserve exact occupancy and metadata")
		for _, room := range rows {
			require.NotEqual(t, foreignRoom.ID, room.ID)
		}
		return queries
	}
	add(0, 3)
	small := run()
	querybudgets.AssertQueryBudget(t, "api.iot.rooms.available", small)
	add(3, 8)
	large := run()
	require.Len(t, large, len(small), "3→8 distinct occupied rooms/sessions/activities/supervisors must stay flat")
	querybudgets.AssertQueryBudget(t, "api.iot.rooms.available", large)
}

func TestKioskDataQueryBudgetTeachers(t *testing.T) {
	t.Parallel()
	db, handler, _, devices := newKioskDataBudgetFixture(t)
	foreign := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreign)
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreign, "Foreign", "Teacher")
	membership, err := owners.NewSchoolMembership(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	_, err = membership.CreateTeacher(tenant.WithTenantID(testpkg.Ctx(t), foreign), schoolmembership.CreateTeacher{
		TeacherFields: schoolmembership.TeacherFields{StaffID: foreignStaff.ID},
	})
	require.NoError(t, err)
	// Teacher rows are returned in teacher.id ASC, NOT staff.id or display_name.
	type orderedTeacher struct {
		teacherID int64
		wire      kioskBudgetTeacher
	}
	want := []orderedTeacher{}
	add := func(first, end int) {
		t.Helper()
		for i := first; i < end; i++ {
			firstName, lastName := fmt.Sprintf("Teacher%02d", 99-i), fmt.Sprintf("Budget%02d", i)
			teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, firstName, lastName)
			require.NotNil(t, teacher.Staff)
			require.NotNil(t, teacher.Staff.Person)
			want = append(want, orderedTeacher{teacher.ID, kioskBudgetTeacher{
				StaffID: teacher.StaffID, PersonID: teacher.Staff.PersonID, FirstName: firstName, LastName: lastName,
				DisplayName: firstName + " " + lastName,
			}})
		}
	}
	run := func() []string {
		t.Helper()
		response, queries := requestKioskData(t, db, handler, devices, "/api/iot/teachers/", false)
		rows := decodeKioskDataRows[kioskBudgetTeacher](t, response, "Available teachers retrieved successfully")
		ordered := slices.Clone(want)
		slices.SortFunc(ordered, func(a, b orderedTeacher) int {
			if a.teacherID < b.teacherID {
				return -1
			}
			if a.teacherID > b.teacherID {
				return 1
			}
			return 0
		})
		expected := make([]kioskBudgetTeacher, len(ordered))
		for i, teacher := range ordered {
			expected[i] = teacher.wire
		}
		require.Equal(t, expected, rows, "exact teacher-order, distinct staff/person IDs and all five wire fields")
		for _, teacher := range rows {
			require.NotEqual(t, foreignStaff.ID, teacher.StaffID)
		}
		return queries
	}
	add(0, 3)
	small := run()
	querybudgets.AssertQueryBudget(t, "api.iot.teachers.list", small)
	add(3, 8)
	large := run()
	require.Len(t, large, len(small), "3→8 distinct teacher/staff/person/account chains must stay flat")
	querybudgets.AssertQueryBudget(t, "api.iot.teachers.list", large)
}

// Real negative authentication paths; never replace auth by injected principals.
// The teachers endpoint is proved PIN-free in its positive budget above.
func TestKioskDataBudgetAuthenticationContracts(t *testing.T) {
	t.Parallel()
	db, handler, _, _ := newKioskDataBudgetFixture(t)
	for _, path := range []string{"/api/iot/activities", "/api/iot/rooms/available", "/api/iot/teachers/"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		counter := testpkg.CaptureQueriesForContext(t, db)
		request = request.WithContext(counter.Context(request.Context()))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
	}
	for _, path := range []string{"/api/iot/activities", "/api/iot/rooms/available"} {
		device := testpkg.CreateTestDevice(t, db, "kiosk-data-no-pin")
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+*device.APIKey)
		counter := testpkg.CaptureQueriesForContext(t, db)
		request = request.WithContext(counter.Context(request.Context()))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
	}
}

// The shared request bucket includes auth discovery/settings, heartbeat and
// handler transactions. No filter removes setters, SELECTs or DML.
