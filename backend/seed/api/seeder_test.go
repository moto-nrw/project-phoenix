package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectSeedState_BasicFields(t *testing.T) {
	s := &Seeder{
		client:  &Client{baseURL: "http://localhost:8080"},
		verbose: false,
	}
	bootstrap := &bootstrapSeedState{
		OrganizationID:   1,
		OrganizationName: "Stadt Koeln",
		OrganizationSlug: "stadt-koeln",
		SchoolID:         2,
		SchoolName:       "GGS Europaschule",
		SchoolSlug:       "ggs-europaschule",
		TenantSlug:       "ggs-europaschule",
		AdminEmail:       "school-admin@example.com",
		AdminPassword:    "Test1234%",
		AdminName:        "Seed Admin",
		AdminPosition:    "OGS-Buero",
	}

	fs := NewFixedSeeder(nil, false)

	// Populate FixedSeeder internal maps
	fs.staffCredentials = []StaffCredentials{
		{Email: "demo1@mail.de", Password: "pass1", PIN: "1000", Name: "Anna Müller", Position: "OGS-Büro"},
		{Email: "demo11@mail.de", Password: "pass11", PIN: "1010", Name: "Julia Klein", Position: "Pädagogische Fachkraft"},
		{Email: "demo12@mail.de", Password: "pass12", PIN: "1011", Name: "Markus Wolf", Position: "Pädagogische Fachkraft"},
	}
	fs.staffIDs = map[string]int64{
		"Anna Müller": 11,
		"Julia Klein": 12,
		"Markus Wolf": 13,
	}
	fs.teacherIDs = map[string]int64{
		"Anna Müller": 10,
		"Julia Klein": 20,
		"Markus Wolf": 30,
	}
	fs.roomIDs = map[string]int64{
		"OGS-Raum 1": 100,
		"Sporthalle": 200,
	}
	fs.activityIDs = map[string]int64{
		"Fußball": 300,
	}
	fs.groupIDs = map[string]int64{
		"sternengruppe": 400,
	}
	fs.deviceKeys = map[string]string{
		"demo-device-001": "apikey-001",
	}
	fs.studentIDByIndex = map[int]int64{
		0: 500,
		1: 501,
	}

	state := s.collectSeedState(fs, "9999", bootstrap)

	assert.Equal(t, "http://localhost:8080", state.BaseURL)
	assert.Equal(t, "9999", state.DevicePIN)
	assert.Equal(t, bootstrap.OrganizationID, state.Bootstrap.OrganizationID)
	assert.Equal(t, bootstrap.OrganizationName, state.Bootstrap.OrganizationName)
	assert.Equal(t, bootstrap.SchoolID, state.Bootstrap.SchoolID)
	assert.Equal(t, bootstrap.TenantSlug, state.Bootstrap.TenantSlug)
	assert.Equal(t, bootstrap.AdminEmail, state.Bootstrap.SchoolAdmin.Email)
	assert.Equal(t, bootstrap.AdminPassword, state.Bootstrap.SchoolAdmin.Password)

	// Admin accounts
	assert.Len(t, state.Accounts.Admin, 1)
	assert.Equal(t, "demo1@mail.de", state.Accounts.Admin[0].Email)
	assert.Equal(t, int64(11), state.Accounts.Admin[0].StaffID)
	assert.Equal(t, int64(10), state.Accounts.Admin[0].TeacherID)

	// Betreuer accounts
	assert.Len(t, state.Accounts.Betreuer, 2)
	assert.Equal(t, "sternengruppe", state.Accounts.Betreuer[0].Group)
	assert.Equal(t, "bärengruppe", state.Accounts.Betreuer[1].Group)
	assert.Equal(t, int64(20), state.Accounts.Betreuer[0].TeacherID)

	// Devices
	assert.Len(t, state.Devices, 1)
	assert.Equal(t, "apikey-001", state.Devices["demo-device-001"].APIKey)

	// Rooms
	assert.Equal(t, int64(100), state.Rooms["OGS-Raum 1"])
	assert.Equal(t, int64(200), state.Rooms["Sporthalle"])

	// Activities
	assert.Equal(t, int64(300), state.Activities["Fußball"])

	// Groups
	assert.Equal(t, int64(400), state.Groups["sternengruppe"])

	// Students - only 2 of DemoStudents[0..1] have IDs in the map
	assert.Len(t, state.Students, 2)
	assert.Equal(t, int64(500), state.Students[0].ID)
	assert.Equal(t, "Felix", state.Students[0].FirstName)
}

func TestCollectSeedState_EmptySeeder(t *testing.T) {
	s := &Seeder{
		client: &Client{baseURL: "http://test:9090"},
	}
	fs := NewFixedSeeder(nil, false)

	state := s.collectSeedState(fs, "0000", nil)

	assert.Equal(t, "http://test:9090", state.BaseURL)
	assert.Equal(t, "0000", state.DevicePIN)
	assert.Zero(t, state.Bootstrap.OrganizationID)
	assert.Empty(t, state.Accounts.Admin)
	assert.Empty(t, state.Accounts.Betreuer)
	assert.Empty(t, state.Devices)
	assert.Empty(t, state.Students)
}

func TestFormatError(t *testing.T) {
	s := &Seeder{verbose: false}
	err := s.formatError("Login", assert.AnError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestNewSeeder(t *testing.T) {
	s := NewSeeder("http://localhost:8080", true)
	assert.NotNil(t, s.client)
	assert.True(t, s.verbose)
	assert.Equal(t, "http://localhost:8080", s.client.baseURL)
}

func TestSeedResult_ZeroValues(t *testing.T) {
	r := &SeedResult{}
	assert.Nil(t, r.Fixed)
}

func TestNewFixedSeeder_InitializesMaps(t *testing.T) {
	fs := NewFixedSeeder(nil, true)
	assert.NotNil(t, fs.roomIDs)
	assert.NotNil(t, fs.personIDs)
	assert.NotNil(t, fs.staffIDs)
	assert.NotNil(t, fs.teacherIDs)
	assert.NotNil(t, fs.studentIDs)
	assert.NotNil(t, fs.studentIDByIndex)
	assert.NotNil(t, fs.studentRFID)
	assert.NotNil(t, fs.groupIDs)
	assert.NotNil(t, fs.activityIDs)
	assert.NotNil(t, fs.activityRoomIDs)
	assert.NotNil(t, fs.categoryIDs)
	assert.NotNil(t, fs.deviceKeys)
	assert.NotNil(t, fs.roleIDs)
	assert.NotNil(t, fs.guardianIDs)
	assert.NotNil(t, fs.staffCredentials)
	assert.True(t, fs.verbose)
}

func TestSeeder_Seed_FullWorkflow(t *testing.T) {
	srv := fullSeedAPIMock(t)
	defer srv.Close()

	// Change to temp dir so output files are written there
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	// Create simulator/iot/ directory for the simulator config
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "simulator", "iot"), 0o755))

	s := NewSeeder(srv.URL, false)
	result, err := s.Seed(context.Background(), "admin@test.de", "pass", "1234")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Fixed)
	assert.Greater(t, result.Fixed.RoomCount, 0)
	assert.Greater(t, result.Fixed.StaffCount, 0)
	assert.Greater(t, result.Fixed.StudentCount, 0)

	// Verify state file was written
	_, err = os.Stat(filepath.Join(tmpDir, DefaultSeedStatePath))
	assert.NoError(t, err)

	// Verify simulator config was written
	_, err = os.Stat(filepath.Join(tmpDir, "simulator", "iot", "simulator.yaml"))
	assert.NoError(t, err)
}

func TestSeeder_Seed_HealthCheckFails(t *testing.T) {
	s := NewSeeder("http://localhost:1", false)
	_, err := s.Seed(context.Background(), "admin@test.de", "pass", "1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Server health check failed")
}

func TestSeeder_Seed_LoginFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/operator/auth/login":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"bad creds"}`)
		}
	}))
	defer srv.Close()

	s := NewSeeder(srv.URL, false)
	_, err := s.Seed(context.Background(), "bad@test.de", "wrong", "1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed")
}

func TestFixedSeeder_Seed_FullWorkflow(t *testing.T) {
	srv := fullSeedAPIMock(t)
	defer srv.Close()

	client := NewClient(srv.URL, false)
	require.NoError(t, client.Login("admin@test.de", "pass"))

	fs := NewFixedSeeder(client, false)
	result, err := fs.Seed(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, len(DemoRooms), result.RoomCount)
	assert.Equal(t, len(DemoStaff), result.PersonCount)
	assert.Equal(t, len(DemoStaff), result.StaffCount)
	assert.Equal(t, len(DemoStaff), result.AccountCount)
	assert.Equal(t, 10, result.GroupCount)
	assert.Equal(t, len(DemoStudents), result.StudentCount)
	assert.Equal(t, len(DemoDevices), result.DeviceCount)
}

func TestPrintSuccessSummary_DoesNotPanic(t *testing.T) {
	s := &Seeder{verbose: false}
	result := &SeedResult{
		Fixed: &FixedResult{
			RoomCount:     12,
			StaffCount:    20,
			AccountCount:  20,
			GroupCount:    10,
			StudentCount:  100,
			GuardianCount: 45,
			ActivityCount: 10,
			DeviceCount:   10,
			StaffCredentials: []StaffCredentials{
				{Name: "Anna Müller", Position: "OGS-Büro", Email: "demo1@mail.de", Password: "pass1", PIN: "1000"},
			},
		},
	}
	// Should not panic
	s.printSuccessSummary("admin@test.de", "generated-password", result)
}

// fullSeedAPIMock creates a comprehensive mock server for the full seed workflow.
func fullSeedAPIMock(t *testing.T) *httptest.Server {
	t.Helper()
	idCounter := int64(0)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idCounter++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/health":
			_, _ = fmt.Fprint(w, `"OK"`)

		case "/operator/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"access_token":  "operator-token",
					"refresh_token": "operator-refresh",
				},
			})

		case "/operator/organizations":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":   1,
					"name": "Demo Organization",
					"slug": "demo-organization",
				},
			})

		case "/operator/schools":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":              1,
					"name":            "Demo School",
					"slug":            "demo-school",
					"subdomain":       "demo-school",
					"active":          true,
					"organization_id": 1,
				},
			})

		case "/operator/schools/1/invite-admin":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":    1,
					"token": "seed-invite-token",
					"email": "school-admin@example.com",
				},
			})

		case "/auth/invitations/seed-invite-token/accept":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"account_id": 1,
					"email":      "school-admin@example.com",
				},
			})

		case "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token":  "test-jwt-token",
				"refresh_token": "refresh",
			})

		case "/auth/roles":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": []map[string]any{
					{"id": 1, "name": "admin"},
					{"id": 2, "name": "user"},
					{"id": 3, "name": "guest"},
				},
			})

		case "/api/activities/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": []map[string]any{
					{"id": 10, "name": "Sport"}, {"id": 11, "name": "Kreativ"},
					{"id": 12, "name": "Hausaufgaben"}, {"id": 13, "name": "Lernen"},
					{"id": 14, "name": "Musik"}, {"id": 15, "name": "Spiele"},
					{"id": 16, "name": "Draußen"}, {"id": 17, "name": "Mensa"},
				},
			})

		case "/api/active/visits":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []any{},
			})

		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":         idCounter,
					"api_key":    fmt.Sprintf("apikey-%d", idCounter),
					"teacher_id": idCounter + 1000,
				},
			})
		}
	}))
}

func TestFixedResult_Fields(t *testing.T) {
	r := &FixedResult{
		RoomCount:        12,
		PersonCount:      20,
		StaffCount:       20,
		AccountCount:     20,
		GroupCount:       10,
		StudentCount:     100,
		SickStudentCount: 5,
		GuardianCount:    45,
		ActivityCount:    10,
		DeviceCount:      10,
		StaffCredentials: []StaffCredentials{
			{Email: "demo1@mail.de", Name: "Anna"},
		},
	}

	assert.Equal(t, 12, r.RoomCount)
	assert.Equal(t, 100, r.StudentCount)
	assert.Len(t, r.StaffCredentials, 1)
}
