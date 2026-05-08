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

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSeedPassword(t *testing.T) {
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

func TestWrapConflictError(t *testing.T) {
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
	s := NewSeeder("http://localhost:8080", false, SeedOptions{})

	token, err := s.extractBootstrapInvitationToken([]byte(`{"data":{"token":"seed-token"}}`))
	require.NoError(t, err)
	assert.Equal(t, "seed-token", token)
}

func TestExtractBootstrapInvitationToken_MissingToken(t *testing.T) {
	s := NewSeeder("http://localhost:8080", false, SeedOptions{})

	token, err := s.extractBootstrapInvitationToken([]byte(`{"data":{}}`))
	require.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "bootstrap invite token not available")
}

func TestMakeBootstrapSeedState_Nil(t *testing.T) {
	bootstrap := makeBootstrapSeedState(nil)
	assert.Zero(t, bootstrap.OrganizationID)
	assert.Empty(t, bootstrap.SchoolAdmin.Email)
}

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
	s := &Seeder{verbose: false}
	err := s.formatError("Login", assert.AnError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestNewSeeder(t *testing.T) {
	s := NewSeeder("http://localhost:8080", true, SeedOptions{})
	assert.NotNil(t, s.client)
	assert.True(t, s.verbose)
	assert.Equal(t, "http://localhost:8080", s.client.baseURL)
}

func TestNewSeeder_WithOptions(t *testing.T) {
	opts := SeedOptions{
		Scenario:      SeedScenarioE2E,
		TenantSlug:    "my-school",
		StaffPassword: "MyPass1!",
		AdminEmail:    "admin@test.com",
	}
	s := NewSeeder("http://localhost:8080", false, opts)
	assert.Equal(t, SeedScenarioE2E, s.options.Scenario)
	require.Error(t, s.optionsErr)
	assert.Contains(t, s.optionsErr.Error(), "owns its deterministic world")
}

func TestApplyScenarioDefaults_E2E(t *testing.T) {
	definition := scenarios.MustLookup(SeedScenarioE2E)
	opts := SeedOptions{Scenario: SeedScenarioE2E}

	err := ApplyScenarioDefaults(&opts)
	require.NoError(t, err)

	assert.Equal(t, definition.Seed.TenantSlug, opts.TenantSlug)
	assert.Equal(t, contract.StatePath, opts.StatePath)
	assert.Equal(t, definition.Seed.StaffPassword, opts.StaffPassword)
	assert.Equal(t, definition.Seed.BootstrapAdminEmail, opts.AdminEmail)
	require.NotNil(t, opts.SecondTenant)
	assert.Equal(t, definition.Seed.SecondTenant.Slug, opts.SecondTenant.Slug)
	assert.Equal(t, definition.Seed.SecondTenant.Name, opts.SecondTenant.Name)
	assert.Equal(t, definition.Seed.SecondTenant.AdminEmail, opts.SecondTenant.AdminEmail)
	assert.Equal(t, definition.Seed.StaffPassword, opts.SecondTenant.AdminPassword)
	assert.Equal(t, definition.Seed.SecondTenant.LinkEmail, opts.SecondTenant.LinkEmail)
}

func TestApplyScenarioDefaults_E2E_RejectsDeterministicOverrides(t *testing.T) {
	opts := SeedOptions{
		Scenario:      SeedScenarioE2E,
		TenantSlug:    "my-school",
		StaffPassword: "MyPass1!",
		AdminEmail:    "admin@test.com",
	}

	err := ApplyScenarioDefaults(&opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owns its deterministic world")
}

func TestApplyScenarioDefaults_E2E_PreservesExplicitStatePath(t *testing.T) {
	opts := SeedOptions{
		Scenario:  SeedScenarioE2E,
		StatePath: "custom-state.json",
	}

	err := ApplyScenarioDefaults(&opts)
	require.NoError(t, err)

	assert.Equal(t, "custom-state.json", opts.StatePath)
}

func TestE2EWorkflow_OnlyContainsScenarioPreparationSteps(t *testing.T) {
	seeder := NewSeeder("http://localhost:8080", false, SeedOptions{
		Scenario: SeedScenarioE2E,
	})

	workflow := e2eWorkflow(seeder)

	require.Equal(t, "e2e-prepare", workflow.Name)
	assert.Equal(t, []string{
		"Server health check",
		"Login",
		"Tenant bootstrap",
		"Stammdaten seeding",
		"Second tenant provisioning",
		"Provisioning E2E check-in fixture",
		"Writing E2E state",
	}, workflowStepNames(workflow))

	assert.False(t, workflowHasStep(workflow, "Marking students sick"))
	assert.False(t, workflowHasStep(workflow, "Seeding privacy consents"))
	assert.False(t, workflowHasStep(workflow, "Seeding announcements"))
	assert.False(t, workflowHasStep(workflow, "Seeding suggestions"))
	assert.False(t, workflowHasStep(workflow, "Writing seed state"))
	assert.False(t, workflowHasStep(workflow, "Generating simulator config"))
	assert.False(t, workflowHasStep(workflow, "Printing summary"))
}

func workflowStepNames(workflow Workflow) []string {
	names := make([]string, 0, len(workflow.Steps))
	for _, step := range workflow.Steps {
		names = append(names, step.Name())
	}
	return names
}

func TestIdentityForScenario_E2E(t *testing.T) {
	definition := scenarios.MustLookup(SeedScenarioE2E)
	identity, err := IdentityForScenario(SeedScenarioE2E)
	require.NoError(t, err)

	assert.Equal(t, definition.Identity.OperatorEmail, identity.OperatorEmail)
	assert.Equal(t, definition.Identity.OperatorPassword, identity.OperatorPassword)
	assert.Equal(t, definition.Identity.OperatorDisplayName, identity.OperatorDisplayName)
	assert.Equal(t, definition.Identity.StaffPIN, identity.StaffPIN)
}

func TestIdentityForScenario_UnknownFails(t *testing.T) {
	identity, err := IdentityForScenario("does-not-exist")
	require.Error(t, err)
	assert.Equal(t, E2EIdentity{}, identity)
	assert.Contains(t, err.Error(), `unknown seed scenario "does-not-exist"`)
}

func TestBuildE2EState(t *testing.T) {
	definition := scenarios.MustLookup(SeedScenarioE2E)
	s := &Seeder{options: SeedOptions{Scenario: SeedScenarioE2E, E2ERuntime: contract.Runtime{
		BackendURL:       "http://localhost:8081",
		TenantDomain:     "localtest.me",
		FrontendPort:     3030,
		OperatorHostname: "operator.localtest.me:3030",
		NextAuthSecret:   "nextauth-secret",
		AuthTrustHost:    true,
	}}}
	rt := &Runtime{
		OperatorEmail:    "operator@e2e.local",
		OperatorPassword: "OperatorPass1!",
		FixedSeeder:      NewFixedSeeder(nil, false, ""),
		StaffPIN:         "1234",
		Bootstrap: &bootstrapSeedState{
			OrganizationID: 1,
			SchoolID:       2,
			TenantSlug:     "demo-school",
			SchoolName:     "Demo School",
		},
		SecondTenant: &SeedStateSecondTenant{
			OrganizationID: 1,
			SchoolID:       3,
			Slug:           "second-school",
			Name:           "Demo School 2",
			LinkEmail:      "demo1@mail.de",
		},
	}
	rt.FixedSeeder.staffCredentials = []StaffCredentials{
		{Email: "demo1@mail.de", Password: "pass1", Name: "Anna Mueller", Position: "OGS-Büro"},
		{Email: "demo11@mail.de", Password: "pass11", Name: "Julia Klein", Position: "Pädagogische Fachkraft"},
	}
	rt.FixedSeeder.staffIDs["Anna Mueller"] = 11
	rt.FixedSeeder.staffIDs["Julia Klein"] = 21
	rt.FixedSeeder.studentIDs["Felix Schneider"] = 100
	rt.FixedSeeder.studentIDs["Emma Meyer"] = 101
	rt.FixedSeeder.studentIDs["Leon Koch"] = 102
	rt.FixedSeeder.roomIDs["OGS-Raum 1"] = 10
	rt.FixedSeeder.activityIDs["Hausaufgaben"] = 20
	rt.FixedSeeder.groupIDs["sternengruppe"] = 30
	rt.FixedSeeder.groupIDs["bärengruppe"] = 40
	rt.FixedSeeder.deviceKeys[DemoDevices[0].DeviceID] = "device-key-123"
	rt.FixedSeeder.studentRFID[100] = definition.Fixtures.Checkin.RFIDTag

	e2e, err := s.buildE2EState(rt)
	require.NoError(t, err)
	require.NotNil(t, e2e)
	assert.Equal(t, contract.StateVersion, e2e.Version)
	assert.Equal(t, "http://localhost:8081", e2e.Runtime.BackendURL)
	assert.Equal(t, "localtest.me", e2e.Runtime.TenantDomain)
	assert.Equal(t, 3030, e2e.Runtime.FrontendPort)
	assert.Equal(t, "operator.localtest.me:3030", e2e.Runtime.OperatorHostname)
	assert.Equal(t, "nextauth-secret", e2e.Runtime.NextAuthSecret)
	assert.True(t, e2e.Runtime.AuthTrustHost)
	assert.Equal(t, SeedScenarioE2E, e2e.World.Scenario.Name)
	assert.Equal(t, "multi-tenant", e2e.World.Scenario.Mode)
	assert.Equal(t, []string{"admin", "staff"}, e2e.Setup.Auth.Roles)
	assert.True(t, e2e.Setup.Auth.RequiresSecondaryTenant)
	assert.True(t, e2e.Setup.Auth.RequiresVerifiedSwitching)
	assert.Equal(t, "demo-school", e2e.World.Tenants.Primary.Slug)
	require.NotNil(t, e2e.World.Tenants.Secondary)
	assert.Equal(t, "second-school", e2e.World.Tenants.Secondary.Slug)
	assert.Equal(t, "demo1@mail.de", e2e.World.Actors.Admin.Email)
	assert.Equal(t, "Julia Klein", e2e.World.Actors.Staff.DisplayName)
	require.NotNil(t, e2e.World.Actors.Operator)
	assert.Equal(t, "operator@e2e.local", e2e.World.Actors.Operator.Email)
	assert.True(t, e2e.Assertions.Switching.Verified)
	require.NotNil(t, e2e.Assertions.Switching.Actor)
	assert.Equal(t, "demo1@mail.de", e2e.Assertions.Switching.Actor.Email)
	assert.Equal(t, DemoDevices[0].DeviceID, e2e.World.Devices.DefaultCheckin.Key)
	assert.Equal(t, "device-key-123", e2e.World.Devices.DefaultCheckin.APIKey)
	assert.Equal(t, "1234", e2e.World.Devices.DefaultCheckin.PIN)
	assert.Equal(t, "demo-device-001", e2e.Fixtures.Checkin.DeviceKey)
	assert.Equal(t, definition.Fixtures.Checkin.RFIDTag, e2e.Fixtures.Checkin.RFIDTag)
	assert.Equal(t, "Hausaufgaben", e2e.Fixtures.Checkin.Activity.Name)
	assert.Equal(t, "OGS-Raum 1", e2e.Fixtures.Checkin.Room.Name)
	assert.Equal(t, int64(11), e2e.Fixtures.Checkin.Supervisor.StaffID)
	assert.Equal(t, "Felix", e2e.Fixtures.Students.SearchPair.Primary.FirstName)
	assert.Equal(t, "Emma", e2e.Fixtures.Students.SearchPair.Secondary.FirstName)
	assert.Equal(t, "Leon", e2e.Fixtures.Students.PresentReady.FirstName)
	assert.Equal(t, "sternengruppe", e2e.Fixtures.Groups.VisiblePair.Primary.Key)
	assert.Equal(t, "Sternengruppe", e2e.Fixtures.Groups.VisiblePair.Primary.DisplayName)
	assert.Equal(t, "bärengruppe", e2e.Fixtures.Groups.VisiblePair.Secondary.Key)
}

func TestBuildE2EState_SwitchingActorMustBeAdmin(t *testing.T) {
	definition := scenarios.MustLookup(SeedScenarioE2E)
	s := &Seeder{options: SeedOptions{Scenario: SeedScenarioE2E}}
	rt := &Runtime{
		OperatorEmail:    "operator@e2e.local",
		OperatorPassword: "OperatorPass1!",
		FixedSeeder:      NewFixedSeeder(nil, false, ""),
		StaffPIN:         "1234",
		Bootstrap: &bootstrapSeedState{
			OrganizationID: 1,
			SchoolID:       2,
			TenantSlug:     "demo-school",
			SchoolName:     "Demo School",
		},
		SecondTenant: &SeedStateSecondTenant{
			OrganizationID: 1,
			SchoolID:       3,
			Slug:           "second-school",
			Name:           "Demo School 2",
			LinkEmail:      "demo1@mail.de",
		},
	}
	rt.FixedSeeder.staffCredentials = []StaffCredentials{
		{Email: "demo2@mail.de", Password: "pass2", Name: "Thomas Schmidt", Position: "OGS-Büro"},
		{Email: "demo11@mail.de", Password: "pass11", Name: "Julia Klein", Position: "Pädagogische Fachkraft"},
	}
	rt.FixedSeeder.staffIDs["Thomas Schmidt"] = 12
	rt.FixedSeeder.staffIDs["Julia Klein"] = 21
	rt.FixedSeeder.studentIDs["Felix Schneider"] = 1
	rt.FixedSeeder.studentIDs["Emma Meyer"] = 2
	rt.FixedSeeder.studentIDs["Leon Koch"] = 3
	rt.FixedSeeder.roomIDs["OGS-Raum 1"] = 10
	rt.FixedSeeder.activityIDs["Hausaufgaben"] = 20
	rt.FixedSeeder.groupIDs["sternengruppe"] = 30
	rt.FixedSeeder.groupIDs["bärengruppe"] = 40
	rt.FixedSeeder.deviceKeys[DemoDevices[0].DeviceID] = "device-key-123"
	rt.FixedSeeder.studentRFID[1] = definition.Fixtures.Checkin.RFIDTag

	e2e, err := s.buildE2EState(rt)
	require.Error(t, err)
	assert.Nil(t, e2e)
	assert.Contains(t, err.Error(), `switching account "demo1@mail.de" is not present`)
}

func TestBuildE2EState_MissingAdminFails(t *testing.T) {
	s := &Seeder{options: SeedOptions{Scenario: SeedScenarioE2E}}
	rt := &Runtime{
		Bootstrap:   &bootstrapSeedState{SchoolID: 2, SchoolName: "Demo School", TenantSlug: "demo-school"},
		FixedSeeder: NewFixedSeeder(nil, false, ""),
		StaffPIN:    "1234",
	}
	rt.FixedSeeder.staffCredentials = []StaffCredentials{
		{Email: "demo11@mail.de", Password: "pass11", Name: "Julia Klein", Position: "Pädagogische Fachkraft"},
	}
	rt.FixedSeeder.staffIDs["Julia Klein"] = 21

	e2e, err := s.buildE2EState(rt)
	require.Error(t, err)
	assert.Nil(t, e2e)
	assert.Contains(t, err.Error(), "no OGS-Büro admin account present")
}

func TestWriteE2EStateStep_MissingState_HardFails(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), ".e2e-state.json")
	seeder := NewSeeder("http://localhost:8080", false, SeedOptions{
		Scenario:  SeedScenarioE2E,
		StatePath: statePath,
	})
	rt := newRuntime(seeder, "operator@e2e.local", "OperatorPass1!", "1234")
	rt.FixedSeeder = NewFixedSeeder(nil, false, "")
	rt.Bootstrap = &bootstrapSeedState{
		OrganizationID: 1,
		SchoolID:       2,
		SchoolName:     "Demo School",
		TenantSlug:     "demo-school",
	}

	err := writeE2EStateStep{seeder: seeder}.Run(context.Background(), rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot build e2e state")
	_, statErr := os.Stat(statePath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestProvisionE2ECheckinFixture(t *testing.T) {
	definition := scenarios.MustLookup(SeedScenarioE2E)
	var sawSessionStart bool
	var sawCheckinRFIDAssign bool
	var sawPresentRFIDAssign bool
	var sawPresentReadyCheckin bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/iot/session/start":
			sawSessionStart = true
			assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
			assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"active_group_id":77}}`)
		case "/api/students/100/rfid":
			sawCheckinRFIDAssign = true
			assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
			assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"student_id":100}}`)
		case "/api/students/102/rfid":
			sawPresentRFIDAssign = true
			assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
			assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"student_id":102}}`)
		case "/api/iot/checkin":
			sawPresentReadyCheckin = true
			assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
			assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"student_id":102,"action":"checked_in","visit_id":88}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	fs := NewFixedSeeder(nil, false, "")
	fs.studentIDs["Felix Schneider"] = 100
	fs.studentIDs["Leon Koch"] = 102
	fs.roomIDs["OGS-Raum 1"] = 10
	fs.activityIDs["Hausaufgaben"] = 20
	fs.deviceKeys[definition.Fixtures.Checkin.DeviceKey] = "device-key-123"
	fs.staffCredentials = []StaffCredentials{
		{
			Email:    "demo1@mail.de",
			Name:     "Anna Mueller",
			Position: "OGS-Büro",
		},
	}
	fs.staffIDs["Anna Mueller"] = 11

	rt := &Runtime{
		Adapter:     phoenixapi.New(srv.URL, false),
		Verbose:     false,
		StaffPIN:    "1234",
		FixedSeeder: fs,
		SecondTenant: &SeedStateSecondTenant{
			LinkEmail: "demo1@mail.de",
		},
	}

	s := &Seeder{options: SeedOptions{Scenario: SeedScenarioE2E}}
	err := s.provisionE2ECheckinFixture(context.Background(), rt)
	require.NoError(t, err)
	assert.True(t, sawSessionStart)
	assert.True(t, sawCheckinRFIDAssign)
	assert.True(t, sawPresentRFIDAssign)
	assert.True(t, sawPresentReadyCheckin)
	assert.Equal(t, definition.Fixtures.Checkin.RFIDTag, fs.studentRFID[100])
	assert.Equal(t, definition.Fixtures.Students.PresentReady.RFIDTag, fs.studentRFID[102])
}

func TestSeedResult_ZeroValues(t *testing.T) {
	r := &SeedResult{}
	assert.Nil(t, r.Fixed)
}

func TestNewFixedSeeder_InitializesMaps(t *testing.T) {
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
	fs := NewFixedSeeder(nil, false, "SharedPass1!")
	assert.Equal(t, "SharedPass1!", fs.staffPassword)
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

	// Verify simulator config was written
	_, err = os.Stat(filepath.Join(tmpDir, "simulator", "iot", "simulator.yaml"))
	assert.NoError(t, err)
}

func TestSeeder_Seed_HealthCheckFails(t *testing.T) {
	s := NewSeeder("http://localhost:1", false, SeedOptions{})
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

	s := NewSeeder(srv.URL, false, SeedOptions{})
	_, err := s.Seed(context.Background(), "bad@test.de", "wrong", "1234")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Login failed")
}

func TestFixedSeeder_Seed_FullWorkflow(t *testing.T) {
	srv := fullSeedAPIMock(t)
	defer srv.Close()

	client := NewClient(srv.URL, false)
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

func TestTruncateSeedSubdomain(t *testing.T) {
	assert.Equal(t, "short", truncateSeedSubdomain("short"))
	long := "this-is-a-very-long-subdomain-that-exceeds-the-sixty-three-character-limit-for-dns-labels"
	assert.Len(t, truncateSeedSubdomain(long), 63)
}
