package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSeedPassword(t *testing.T) {
	t.Parallel()

	// Run multiple times to verify deterministic compliance (was probabilistic before fix).
	for i := 0; i < 50; i++ {
		password, err := generateSeedPassword()
		require.NoError(t, err)
		assert.Len(t, password, seedPasswordLength)
		for _, char := range password {
			assert.Contains(t, seedPasswordAlphabet, string(char))
		}
		// Every generated password must satisfy ValidatePasswordStrength.
		assert.Regexp(t, `[a-z]`, password, "must contain lowercase")
		assert.Regexp(t, `[A-Z]`, password, "must contain uppercase")
		assert.Regexp(t, `[0-9]`, password, "must contain digit")
		assert.Regexp(t, `[^a-zA-Z0-9]`, password, "must contain special char")
	}
}

func TestParentEnrollmentSeedSettingsDisableCaptcha(t *testing.T) {
	t.Parallel()

	seen := make(map[string]any)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)

		var body struct {
			Value any `json:"value"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		seen[strings.TrimPrefix(r.URL.Path, "/api/settings/values/")] = body.Value

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	}))
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	step := parentEnrollmentSeedStep{}

	settings, err := step.seedSettings(rt, phoenixapi.AuthRef{})
	require.NoError(t, err)

	assert.Equal(t, false, settings[configModels.KeyEnrollmentRequireCaptcha])
	assert.Equal(t, false, seen[configModels.KeyEnrollmentRequireCaptcha])
}

func TestParentEnrollmentCareOfferingsIncludePickupBaselines(t *testing.T) {
	t.Parallel()

	type offeringPayload struct {
		Name          string            `json:"name"`
		AvailableDays []string          `json:"available_days"`
		CountsAsCare  bool              `json:"counts_as_care"`
		PickupTimes   map[string]string `json:"pickup_times"`
	}
	var received []offeringPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/enrollment/care-offerings", r.URL.Path)
		var body offeringPayload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		received = append(received, body)
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%d"}}`, len(received))
	}))
	defer srv.Close()

	step := parentEnrollmentSeedStep{}
	_, err := step.createCareOfferings(&Runtime{Client: newTestClient(srv.URL, false)}, phoenixapi.AuthRef{}, 1)
	require.NoError(t, err)
	require.Len(t, received, 4)

	expectedTimes := map[string]string{"OGS Ganztag": "16:00", "Kurzbetreuung": "14:00", "Mittagessen": "", "Ferienbetreuung Herbst": "16:00"}
	for _, offering := range received {
		expectedTime, ok := expectedTimes[offering.Name]
		require.True(t, ok, offering.Name)
		countsAsCare := expectedTime != ""
		assert.Equal(t, countsAsCare, offering.CountsAsCare, offering.Name)
		if !countsAsCare {
			assert.Empty(t, offering.PickupTimes, offering.Name)
			continue
		}
		assert.Len(t, offering.PickupTimes, len(offering.AvailableDays), offering.Name)
		for _, day := range offering.AvailableDays {
			assert.Equal(t, expectedTime, offering.PickupTimes[day], "%s %s", offering.Name, day)
		}
	}
}

func TestParentEnrollmentParentPassword(t *testing.T) {
	t.Parallel()

	step := parentEnrollmentSeedStep{seeder: &Seeder{}}

	password, err := step.parentPassword()
	require.NoError(t, err)
	assert.Equal(t, defaultSeedParentPassword, password)

	step = parentEnrollmentSeedStep{seeder: &Seeder{options: SeedOptions{StaffPassword: "SharedPass1!"}}}
	password, err = step.parentPassword()
	require.NoError(t, err)
	assert.Equal(t, "SharedPass1!", password)
}

func TestPublicEnrollmentSeedHeadersUseDistinctForwardedIPs(t *testing.T) {
	t.Parallel()

	first := publicEnrollmentSeedHeaders(0)
	second := publicEnrollmentSeedHeaders(1)

	assert.Equal(t, "198.51.100.10", first["X-Forwarded-For"])
	assert.Equal(t, "198.51.100.11", second["X-Forwarded-For"])
	assert.NotEqual(t, first["X-Forwarded-For"], second["X-Forwarded-For"])
}

func TestClientPostPublicWithHeaders(t *testing.T) {
	t.Parallel()

	var forwardedFor string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedFor = r.Header.Get("X-Forwarded-For")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	_, err := client.PostPublicWithHeaders("/api/enrollment/demo-school/submit", map[string]any{}, map[string]string{
		"X-Forwarded-For": "198.51.100.42",
	})
	require.NoError(t, err)
	assert.Equal(t, "198.51.100.42", forwardedFor)
}

func TestWrapConflictError(t *testing.T) {
	t.Parallel()

	t.Run("wraps 409 APIError with helpful message", func(t *testing.T) {
		apiErr := &phoenixapi.APIError{
			Method:     "POST",
			Path:       "/operator/organizations",
			StatusCode: http.StatusConflict,
			Message:    "slug already taken",
		}
		err := wrapConflictError(apiErr, "organization")
		assert.Contains(t, err.Error(), "seed organization already exists (409 Conflict)")
		assert.Contains(t, err.Error(), "migrate reset")
	})

	t.Run("passes through non-409 errors unchanged", func(t *testing.T) {
		apiErr := &phoenixapi.APIError{
			Method:     "POST",
			Path:       "/operator/organizations",
			StatusCode: http.StatusInternalServerError,
			Message:    "internal error",
		}
		err := wrapConflictError(apiErr, "organization")
		assert.Equal(t, apiErr, err)
	})

	t.Run("passes through non-API errors unchanged", func(t *testing.T) {
		original := fmt.Errorf("network timeout")
		err := wrapConflictError(original, "school")
		assert.Equal(t, original, err)
	})
}

func TestExtractBootstrapInvitationToken(t *testing.T) {
	t.Parallel()

	s := NewSeeder("http://localhost:8080", false, SeedOptions{})

	token, err := s.extractBootstrapInvitationToken([]byte(`{"data":{"token":"seed-token"}}`))
	require.NoError(t, err)
	assert.Equal(t, "seed-token", token)
}

func TestExtractBootstrapInvitationToken_MissingToken(t *testing.T) {
	t.Parallel()

	s := NewSeeder("http://localhost:8080", false, SeedOptions{})

	token, err := s.extractBootstrapInvitationToken([]byte(`{"data":{}}`))
	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "bootstrap invite token not available")
}

func TestMakeBootstrapSeedState_Nil(t *testing.T) {
	t.Parallel()

	bootstrap := makeBootstrapSeedState(nil)
	assert.Zero(t, bootstrap.OrganizationID)
	assert.Empty(t, bootstrap.SchoolAdmin.Email)
}

func TestCollectSeedState_BasicFields(t *testing.T) {
	t.Parallel()

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

	fs := NewFixedSeeder(nil, false, "")

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
	t.Parallel()

	s := &Seeder{
		client: &Client{baseURL: "http://test:9090"},
	}
	fs := NewFixedSeeder(nil, false, "")

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
	t.Parallel()

	s := &Seeder{verbose: false}
	err := s.formatError("Login", assert.AnError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestNewSeeder(t *testing.T) {
	t.Parallel()

	s := NewSeeder("http://localhost:8080", true, SeedOptions{})
	assert.NotNil(t, s.client)
	assert.True(t, s.verbose)
	assert.Equal(t, "http://localhost:8080", s.client.baseURL)
}

func TestNewSeeder_WithOptions(t *testing.T) {
	t.Parallel()

	opts := SeedOptions{
		TenantSlug:    "my-school",
		StaffPassword: "MyPass1!",
		AdminEmail:    "admin@test.com",
	}
	s := NewSeeder("http://localhost:8080", false, opts)
	assert.Equal(t, "my-school", s.options.TenantSlug)
	assert.Equal(t, "MyPass1!", s.options.StaffPassword)
	assert.Equal(t, "admin@test.com", s.options.AdminEmail)
}

func TestSeedResult_ZeroValues(t *testing.T) {
	t.Parallel()

	r := &SeedResult{}
	assert.Nil(t, r.Fixed)
}

func TestNewFixedSeeder_InitializesMaps(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, true, "")
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
	assert.Empty(t, fs.staffPassword)
}

func TestNewFixedSeeder_WithStaffPassword(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(nil, false, "SharedPass1!")
	assert.Equal(t, "SharedPass1!", fs.staffPassword)
}

// Deliberately NOT parallel: mutates process-global configuration.
func TestSeeder_Seed_FullWorkflow(t *testing.T) {
	srv := fullSeedAPIMock(t)
	defer srv.Close()

	// Change to temp dir so output files are written there
	origDir, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origDir) }()

	s := NewSeeder(srv.URL, false, SeedOptions{})
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
}

func TestSeeder_Seed_HealthCheckFails(t *testing.T) {
	t.Parallel()

	s := NewSeeder("http://localhost:1", false, SeedOptions{})
	_, err := s.Seed(context.Background(), "admin@test.de", "pass", "1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Server health check failed")
}

func TestSeeder_Seed_LoginFails(t *testing.T) {
	t.Parallel()

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

	s := NewSeeder(srv.URL, false, SeedOptions{})
	_, err := s.Seed(context.Background(), "bad@test.de", "wrong", "1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed")
}

func TestFixedSeeder_Seed_FullWorkflow(t *testing.T) {
	t.Parallel()

	srv := fullSeedAPIMock(t)
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	require.NoError(t, client.Login("admin@test.de", "pass"))

	fs := NewFixedSeeder(client, false, "")
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
	t.Parallel()

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
	state := &SeedState{
		Parents: []ParentCredentials{
			{Email: "parent@example.test", Password: "pass1", Name: "Demo Parent", StudentIDs: []int64{42}},
		},
		Enrollment: SeedEnrollmentState{
			PhaseID:   1,
			Offerings: map[string]int64{"ogs-ganztag": 2},
			Requests: []SeedEnrollmentRequest{
				{RequestID: 1, Source: "public", Status: "approved", ChildIDs: []int64{7}},
				{RequestID: 2, Source: "parent", ChildIDs: []int64{8}},
			},
			ParentActions: []SeedParentPortalAction{
				{ParentEmail: "parent@example.test", StudentID: 42, Type: "note"},
			},
		},
	}
	s.printSuccessSummary("admin@test.de", "generated-password", result, state)
}

// fullSeedAPIMock creates a comprehensive mock server for the full seed workflow.
func fullSeedAPIMock(t *testing.T) *httptest.Server {
	t.Helper()
	idCounter := int64(0)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idCounter++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if strings.HasPrefix(r.URL.Path, "/api/guardians/") && strings.HasSuffix(r.URL.Path, "/invite") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":                  idCounter,
					"guardian_profile_id": idCounter,
					"token":               fmt.Sprintf("guardian-invite-%d", idCounter),
					"email_sent":          true,
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/auth/guardian-invitations/") && strings.HasSuffix(r.URL.Path, "/accept") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"account_id": idCounter,
					"email":      "parent@example.test",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/enrollment/admin/requests/") && strings.Contains(r.URL.Path, "/children/") && strings.HasSuffix(r.URL.Path, "/decide") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":            fmt.Sprintf("%d", idCounter),
					"first_name":    "Demo",
					"last_name":     "Child",
					"date_of_birth": "2019-01-01",
					"status":        "approved",
				},
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/enrollment/admin/requests/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id":           fmt.Sprintf("%d", idCounter),
					"status_token": fmt.Sprintf("status-token-%d", idCounter),
					"children": []map[string]any{
						{"id": fmt.Sprintf("%d", idCounter+1000)},
					},
				},
			})
			return
		}

		switch r.URL.Path {
		case "/health":
			_, _ = fmt.Fprint(w, `"OK"`)

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

		case "/parent/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"access_token":  "parent-token",
					"refresh_token": "parent-refresh",
				},
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

		case "/api/enrollment/phases":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id": "1001",
				},
			})

		case "/api/enrollment/care-offerings":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id": fmt.Sprintf("%d", idCounter),
				},
			})

		case "/api/files/audience":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"roles":    []map[string]any{{"id": "2", "name": "Betreuungskraft"}},
					"accounts": []map[string]any{{"account_id": "41"}, {"account_id": "42"}},
				},
			})

		case "/api/files/folders":
			// Ids leave this API as decimal strings (#2596).
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"id": fmt.Sprintf("%d", idCounter),
				},
			})

		case "/api/enrollment/demo-school/submit", "/parent/enrollments/demo-school/submit":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"request_id": fmt.Sprintf("%d", idCounter),
					"status_url": fmt.Sprintf("https://parents.example.test/status/status-token-%d", idCounter),
				},
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
	t.Parallel()

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

func TestTruncateSeedSubdomain(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "short", truncateSeedSubdomain("short"))
	long := "this-is-a-very-long-subdomain-that-exceeds-the-sixty-three-character-limit-for-dns-labels"
	assert.Len(t, truncateSeedSubdomain(long), 63)
}
