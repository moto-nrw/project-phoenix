package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiMock creates a mock server that responds to various seed API endpoints.
func apiMock(t *testing.T) *httptest.Server {
	t.Helper()
	idCounter := int64(0)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idCounter++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token":  "test-jwt-token",
				"refresh_token": "test-refresh",
			})

		case "/auth/roles":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": []map[string]any{
					{"id": 10, "name": "admin"},
					{"id": 20, "name": "user"},
					{"id": 30, "name": "guest"},
				},
			})

		case "/api/activities/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": []map[string]any{
					{"id": 10, "name": "Sport"},
					{"id": 11, "name": "Kreativ"},
					{"id": 12, "name": "Hausaufgaben"},
					{"id": 13, "name": "Lernen"},
					{"id": 14, "name": "Musik"},
					{"id": 15, "name": "Spiele"},
					{"id": 16, "name": "Draußen"},
					{"id": 17, "name": "Mensa"},
					{"id": 18, "name": "Gruppenraum"},
				},
			})

		case "/api/active/visits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []any{},
			})

		case "/auth/register":
			// Mirrors the real endpoint: a staff-tier role provisions the
			// person and staff record with the account and reports their ids,
			// as JSON strings because they are bigints (#2222).
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id": idCounter,
					"school_identity": map[string]any{
						"person_id": fmt.Sprintf("%d", idCounter+2000),
						"staff_id":  fmt.Sprintf("%d", idCounter+3000),
					},
				},
			})

		default:
			// Generic success with ID for POST/PUT (rooms, staff, students, etc.)
			resp := map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":         idCounter,
					"api_key":    fmt.Sprintf("apikey-%d", idCounter),
					"teacher_id": idCounter + 1000,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
}

func TestFixedSeeder_FetchRoles(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	err := fs.fetchRoles(context.TODO())
	require.NoError(t, err)

	assert.Equal(t, int64(10), fs.roleIDs["admin"])
	assert.Equal(t, int64(20), fs.roleIDs["user"])
	assert.Equal(t, int64(30), fs.roleIDs["guest"])
}

func TestFixedSeeder_SeedRooms(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")
	result := &FixedResult{}

	err := fs.seedRooms(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoRooms), result.RoomCount)
	assert.Len(t, fs.roomIDs, len(DemoRooms))
}

func TestFixedSeeder_SeedStaff(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate personIDs (normally done by seedPersons)
	for _, staff := range DemoStaff {
		key := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		fs.personIDs[key] = int64(len(fs.personIDs) + 1)
	}

	result := &FixedResult{}
	err := fs.seedStaff(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoStaff), result.StaffCount)
	assert.Len(t, fs.staffIDs, len(DemoStaff))
}

func TestFixedSeeder_SeedStaff_MissingPerson(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	// personIDs is empty, so should fail
	result := &FixedResult{}
	err := fs.seedStaff(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "person not found")
}

func TestFixedSeeder_SeedGroups(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")
	// Pre-populate teacherIDs for group assignment
	fs.teacherIDs["Julia Klein"] = 100
	fs.teacherIDs["Markus Wolf"] = 200

	result := &FixedResult{}
	err := fs.seedGroups(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, 10, result.GroupCount)
	assert.Len(t, fs.groupIDs, 10)
}

func TestFixedSeeder_SeedStudents(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate groupIDs
	groupKeys := []string{
		"sternengruppe", "bärengruppe", "sonnengruppe", "mondgruppe",
		"regenbogengruppe", "blumengruppe", "schmetterlingsgruppe",
		"waldgruppe", "meeresgruppe", "wiesengruppe",
	}
	for i, key := range groupKeys {
		fs.groupIDs[key] = int64(i + 1)
	}

	result := &FixedResult{}
	err := fs.seedStudents(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoStudents), result.StudentCount)
	assert.Len(t, fs.studentIDs, len(DemoStudents))
	assert.Len(t, fs.studentIDByIndex, len(DemoStudents))
}

func TestFixedSeeder_SeedStudents_MissingGroup(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	// groupIDs is empty
	result := &FixedResult{}
	err := fs.seedStudents(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group not found")
}

func TestFixedSeeder_SeedGuardians(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate studentIDByIndex for all 100 students
	for i := 0; i < 100; i++ {
		fs.studentIDByIndex[i] = int64(i + 100)
	}

	result := &FixedResult{}
	err := fs.seedGuardians(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoGuardians), result.GuardianCount)
}

func TestFixedSeeder_FetchCategories(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")
	err := fs.fetchCategories(context.TODO())
	require.NoError(t, err)
	assert.NotEmpty(t, fs.categoryIDs)
	assert.Equal(t, int64(10), fs.categoryIDs["Sport"])
}

func TestFixedSeeder_FetchCategories_Empty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   []any{},
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	err := fs.fetchCategories(context.TODO())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no categories found")
}

func TestFixedSeeder_SeedActivities(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate roomIDs and categoryIDs
	for _, act := range DemoActivities {
		fs.roomIDs[act.DefaultRoom] = int64(len(fs.roomIDs) + 1)
	}
	fs.categoryIDs["Sport"] = 10
	fs.categoryIDs["Kreativ"] = 11
	fs.categoryIDs["Hausaufgaben"] = 12
	fs.categoryIDs["Lernen"] = 13
	fs.categoryIDs["Musik"] = 14
	fs.categoryIDs["Spiele"] = 15
	fs.categoryIDs["Draußen"] = 16
	fs.categoryIDs["Mensa"] = 17

	result := &FixedResult{}
	err := fs.seedActivities(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoActivities), result.ActivityCount)
}

func TestFixedSeeder_SeedActivities_MissingRoom(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	fs.categoryIDs["Sport"] = 1
	// roomIDs is empty
	result := &FixedResult{}
	err := fs.seedActivities(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "room not found")
}

func TestFixedSeeder_AssignSupervisors(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate staffIDs and activityIDs
	firstStaffKey := fmt.Sprintf("%s %s", DemoStaff[0].FirstName, DemoStaff[0].LastName)
	fs.staffIDs[firstStaffKey] = 1
	fs.activityIDs["Fußball"] = 100
	fs.activityIDs["Basteln"] = 101

	err := fs.assignSupervisors(context.TODO())
	require.NoError(t, err)
}

func TestFixedSeeder_AssignSupervisors_NoStaff(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, false, "")
	err := fs.assignSupervisors(context.TODO())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no staff available")
}

func TestFixedSeeder_EnrollStudents(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate activityIDs and studentIDs
	fs.activityIDs["Fußball"] = 100
	for i, s := range DemoStudents[:10] {
		key := fmt.Sprintf("%s %s", s.FirstName, s.LastName)
		fs.studentIDs[key] = int64(i + 1)
	}

	err := fs.enrollStudents(context.TODO())
	require.NoError(t, err)
}

func TestFixedSeeder_SeedDevices(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")
	result := &FixedResult{}

	err := fs.seedDevices(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoDevices), result.DeviceCount)
	assert.Len(t, fs.deviceKeys, len(DemoDevices))
}

func TestFixedSeeder_SeedStaffAccounts(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	fs.roleIDs["admin"] = 1
	fs.roleIDs["user"] = 2
	fs.roleIDs["guest"] = 3

	// Pre-populate personIDs
	for _, staff := range DemoStaff {
		key := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		fs.personIDs[key] = int64(len(fs.personIDs) + 1)
	}

	result := &FixedResult{}
	err := fs.seedStaffAccounts(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, len(DemoStaff), result.AccountCount)
	assert.Len(t, fs.staffCredentials, len(DemoStaff))

	// Verify first credential
	assert.Equal(t, "demo1@mail.de", fs.staffCredentials[0].Email)
	assert.Equal(t, "OGS-Büro", fs.staffCredentials[0].Position)
}

func TestFixedSeeder_SeedStaffAccounts_MissingRole(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, false, "")
	// No roles
	result := &FixedResult{}
	err := fs.seedStaffAccounts(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin role not found")
}

func TestFixedSeeder_SeedStaffAccounts_MissingUserRole(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, false, "")
	fs.roleIDs["admin"] = 1
	// No "user" role
	result := &FixedResult{}
	err := fs.seedStaffAccounts(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user role not found")
}

func TestFixedSeeder_SeedStaffAccounts_MissingGuestRole(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, false, "")
	fs.roleIDs["admin"] = 1
	fs.roleIDs["user"] = 2
	// No "guest" role
	result := &FixedResult{}
	err := fs.seedStaffAccounts(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "guest role not found")
}

func TestFixedSeeder_SwitchToStaffAccount(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	fs.staffCredentials = []StaffCredentials{
		{Email: "demo1@mail.de", Password: "pass1", Name: "Anna Müller", Position: "OGS-Büro"},
	}

	err := fs.switchToStaffAccount()
	require.NoError(t, err)
}

func TestFixedSeeder_SwitchToStaffAccount_NoOGSStaff(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, false, "")
	fs.staffCredentials = []StaffCredentials{
		{Position: "Pädagogische Fachkraft"},
	}

	err := fs.switchToStaffAccount()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no OGS-Büro staff credential found")
}

func TestFixedSeeder_MarkStudentsSick(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate studentIDByIndex
	for i := 0; i < len(DemoStudents); i++ {
		fs.studentIDByIndex[i] = int64(i + 100)
	}

	result := &FixedResult{}
	err := fs.MarkStudentsSick(context.TODO(), result)
	require.NoError(t, err)
	// Should mark some students sick (at least some groups should get sick students)
	assert.Greater(t, result.SickStudentCount, 0)
}

func TestFixedSeeder_GetCheckedInStudentIDs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": []map[string]any{
				{"student_id": 1},
				{"student_id": 5},
				{"student_id": 10},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	ids, err := fs.getCheckedInStudentIDs()
	require.NoError(t, err)
	assert.True(t, ids[1])
	assert.True(t, ids[5])
	assert.True(t, ids[10])
	assert.False(t, ids[2])
}

func TestFixedSeeder_GetCheckedInStudentIDs_Empty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   []any{},
		})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, false, "")
	ids, err := fs.getCheckedInStudentIDs()
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// =============================================================================
// Demo Data Validation Tests
// =============================================================================

func TestDemoData_RoomsHaveRequiredFields(t *testing.T) {
	t.Parallel()

	for _, room := range DemoRooms {
		assert.NotEmpty(t, room.Name, "room must have a name")
		assert.NotEmpty(t, room.Category, "room %s must have a category", room.Name)
		assert.Greater(t, room.Capacity, 0, "room %s must have positive capacity", room.Name)
	}
}

func TestDemoData_StaffHaveRequiredFields(t *testing.T) {
	t.Parallel()

	for _, staff := range DemoStaff {
		assert.NotEmpty(t, staff.FirstName)
		assert.NotEmpty(t, staff.LastName)
		assert.Contains(t, []string{"OGS-Büro", "Pädagogische Fachkraft", "Extern"}, staff.Position)
	}
}

func TestDemoData_StudentsHaveRequiredFields(t *testing.T) {
	t.Parallel()

	validGroups := map[string]bool{
		"sternengruppe": true, "bärengruppe": true, "sonnengruppe": true,
		"mondgruppe": true, "regenbogengruppe": true, "blumengruppe": true,
		"schmetterlingsgruppe": true, "waldgruppe": true, "meeresgruppe": true,
		"wiesengruppe": true,
	}

	for _, student := range DemoStudents {
		assert.NotEmpty(t, student.FirstName)
		assert.NotEmpty(t, student.LastName)
		assert.NotEmpty(t, student.Class)
		assert.True(t, validGroups[student.GroupKey], "invalid group key: %s", student.GroupKey)
	}
}

func TestDemoData_ActivitiesHaveRequiredFields(t *testing.T) {
	t.Parallel()

	for _, act := range DemoActivities {
		assert.NotEmpty(t, act.Name)
		assert.NotEmpty(t, act.DefaultRoom)
		assert.Greater(t, act.DurationMins, 0)
	}
}

func TestDemoData_DevicesHaveRequiredFields(t *testing.T) {
	t.Parallel()

	for _, device := range DemoDevices {
		assert.NotEmpty(t, device.DeviceID)
		assert.NotEmpty(t, device.Name)
	}
}

func TestDemoData_GuardiansHaveRequiredFields(t *testing.T) {
	t.Parallel()

	for _, guardian := range DemoGuardians {
		assert.NotEmpty(t, guardian.FirstName)
		assert.NotEmpty(t, guardian.LastName)
		assert.Contains(t, []string{"parent", "guardian", "relative", "other"}, guardian.Relationship)
		assert.GreaterOrEqual(t, guardian.StudentIndex, 0)
		assert.Less(t, guardian.StudentIndex, len(DemoStudents))
	}
}

func TestDemoData_Counts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 11, len(DemoRooms))
	assert.Equal(t, 20, len(DemoStaff))
	assert.Equal(t, 100, len(DemoStudents))
	assert.Equal(t, 8, len(DemoActivities))
	assert.Equal(t, 10, len(DemoDevices))
}

func TestFixedSeeder_SeedRooms_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"db error"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, false, "")
	result := &FixedResult{}
	err := fs.seedRooms(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create room")
}

func TestFixedSeeder_SeedDevices_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"db error"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, false, "")
	result := &FixedResult{}
	err := fs.seedDevices(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create device")
}

func TestFixedSeeder_FetchRoles_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"db error"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, false, "")
	err := fs.fetchRoles(context.TODO())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch roles")
}

func TestFixedSeeder_FetchCategories_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"db error"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, false, "")
	err := fs.fetchCategories(context.TODO())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch categories")
}

func TestFixedSeeder_SeedGuardians_MissingStudentIndex(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, true, "") // verbose to cover warning branch
	// studentIDByIndex is empty, so no students can be linked
	result := &FixedResult{}
	err := fs.seedGuardians(context.TODO(), result)
	require.NoError(t, err)
	// Count should be 0 since no students were linkable
	assert.Equal(t, 0, result.GuardianCount)
}

func TestFixedSeeder_EnrollStudents_MissingStudentID(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, true, "") // verbose to cover warning branch
	fs.activityIDs["Fußball"] = 100
	// studentIDs is empty, so no students can be enrolled

	err := fs.enrollStudents(context.TODO())
	require.NoError(t, err)
}

func TestFixedSeeder_SwitchToStaffAccount_LoginError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"bad creds"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	fs := NewFixedSeeder(client, false, "")
	fs.staffCredentials = []StaffCredentials{
		{Email: "demo1@mail.de", Password: "wrong", Name: "Admin", Position: "OGS-Büro"},
	}

	err := fs.switchToStaffAccount()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to login")
}

func TestFixedSeeder_SeedRooms_InvalidResponseJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, false, "")
	result := &FixedResult{}
	err := fs.seedRooms(context.TODO(), result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse room response")
}

func TestFixedSeeder_GetCheckedInStudentIDs_APIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"db error"}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test"

	fs := NewFixedSeeder(client, false, "")
	_, err := fs.getCheckedInStudentIDs()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch active visits")
}

func TestFloor_Helper(t *testing.T) {
	t.Parallel()

	f := floor(2)
	assert.NotNil(t, f)
	assert.Equal(t, 2, *f)

	f0 := floor(0)
	assert.NotNil(t, f0)
	assert.Equal(t, 0, *f0)
}

func TestFixedSeeder_SeedPickupSchedules(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")

	// Pre-populate all 100 students
	for i := 0; i < 100; i++ {
		fs.studentIDByIndex[i] = int64(i + 100)
	}

	result := &FixedResult{}
	err := fs.seedPickupSchedules(context.TODO(), result)
	require.NoError(t, err)
	// Every other student (odd indices) gets a schedule: 100/2 = 50
	assert.Equal(t, 50, result.PickupScheduleCount)
}

func TestFixedSeeder_SeedPickupSchedules_EmptyStudents(t *testing.T) {
	t.Parallel()

	srv := apiMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"

	fs := NewFixedSeeder(client, true, "")
	// studentIDByIndex is empty — no schedules should be created

	result := &FixedResult{}
	err := fs.seedPickupSchedules(context.TODO(), result)
	require.NoError(t, err)
	assert.Equal(t, 0, result.PickupScheduleCount)
}

func TestFixedSeeder_SeedClassArrivalTimes(t *testing.T) {
	t.Parallel()

	var requests []struct {
		SchoolClass string `json:"school_class"`
		Schedules   []struct {
			Weekday int    `json:"weekday"`
			Time    string `json:"expected_arrival"`
		} `json:"schedules"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/api/students/arrival-schedules/bulk", r.URL.Path)
		request := struct {
			SchoolClass string `json:"school_class"`
			Schedules   []struct {
				Weekday int    `json:"weekday"`
				Time    string `json:"expected_arrival"`
			} `json:"schedules"`
		}{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		requests = append(requests, request)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	client.token = "test-token"
	fs := NewFixedSeeder(client, true, "")
	result := &FixedResult{}

	err := fs.seedClassArrivalTimes(context.TODO(), result)

	require.NoError(t, err)
	assert.Equal(t, 8, result.ClassArrivalTimeCount)
	require.Len(t, requests, 8)
	assert.Equal(t, "Klasse 1a", requests[0].SchoolClass)
	assert.Equal(t, "Klasse 4b", requests[7].SchoolClass)
	for _, request := range requests {
		assert.Len(t, request.Schedules, 5)
		assert.Equal(t, "11:45", request.Schedules[0].Time)
		assert.Equal(t, "12:45", request.Schedules[2].Time)
	}
}
