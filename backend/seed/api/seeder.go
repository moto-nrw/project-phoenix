package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

const (
	defaultSeedOrganizationName = "Demo Organization"
	defaultSeedOrganizationSlug = "demo-organization"
	defaultSeedSchoolName       = "Demo School"
	defaultSeedSchoolSlug       = "demo-school"
	defaultSeedSchoolSubdomain  = "demo-school"
	seedTokenHeader             = "X-Phoenix-Seed-Token"
	defaultSeedPassword         = "Test1234%"
	defaultSeedAdminEmail       = "school-admin@example.com"
)

// Seeder orchestrates the complete API-based seeding process
type Seeder struct {
	adapter *phoenixapi.Adapter
	client  *Client
	verbose bool
}

// SeedResult contains counts of created entities
type SeedResult struct {
	Fixed *FixedResult
}

type bootstrapSeedState struct {
	OrganizationID   int64
	OrganizationName string
	OrganizationSlug string
	SchoolID         int64
	SchoolName       string
	SchoolSlug       string
	TenantSlug       string
	AdminEmail       string
	AdminPassword    string
	AdminName        string
	AdminPosition    string
}

// NewSeeder creates a new API seeder
func NewSeeder(baseURL string, verbose bool) *Seeder {
	adapter := phoenixapi.New(baseURL, verbose)
	return &Seeder{
		adapter: adapter,
		client:  NewClientWithAdapter(adapter, verbose),
		verbose: verbose,
	}
}

// Seed executes the complete seeding workflow
func (s *Seeder) Seed(ctx context.Context, email, password, staffPIN string) (*SeedResult, error) {
	runtime := newRuntime(s, email, password, staffPIN)
	workflow := fullDemoWorkflow(s)
	if err := workflow.Run(ctx, runtime); err != nil {
		var stepErr *StepError
		if errors.As(err, &stepErr) {
			return nil, s.formatError(stepErr.Step, stepErr.Err)
		}
		return nil, s.formatError(workflow.Name, err)
	}
	return runtime.Result, nil
}

// bootstrapTenant creates the demo organization, school, and admin account with
// deterministic credentials. This assumes a clean database (migrate reset) —
// re-running without reset will fail with 409 due to duplicate slugs/emails.
func (s *Seeder) bootstrapTenant(ctx context.Context) (*bootstrapSeedState, error) {
	orgID, err := s.createSeedOrganization(defaultSeedOrganizationName, defaultSeedOrganizationSlug)
	if err != nil {
		return nil, wrapConflictError(err, "organization")
	}

	schoolID, tenantSlug, err := s.createSeedSchool(orgID, defaultSeedSchoolName, defaultSeedSchoolSlug, defaultSeedSchoolSubdomain)
	if err != nil {
		return nil, wrapConflictError(err, "school")
	}

	adminEmail := defaultSeedAdminEmail
	adminPassword := defaultSeedPassword
	adminName := "Seed Admin"
	adminPosition := "OGS-Büro"
	inviteResp, err := s.client.PostWithHeaders(fmt.Sprintf("/operator/schools/%d/invite-admin", schoolID), map[string]any{
		"email":      adminEmail,
		"first_name": "Seed",
		"last_name":  "Admin",
		"position":   adminPosition,
	}, map[string]string{
		seedTokenHeader: "true",
	})
	if err != nil {
		return nil, fmt.Errorf("invite school admin: %w", err)
	}
	token, err := s.extractBootstrapInvitationToken(inviteResp)
	if err != nil {
		return nil, err
	}

	if _, err := s.client.PostPublic(fmt.Sprintf("/auth/invitations/%s/accept", token), map[string]any{
		"password":         adminPassword,
		"confirm_password": adminPassword,
	}); err != nil {
		return nil, fmt.Errorf("accept invitation: %w", err)
	}

	return &bootstrapSeedState{
		OrganizationID:   orgID,
		OrganizationName: defaultSeedOrganizationName,
		OrganizationSlug: defaultSeedOrganizationSlug,
		SchoolID:         schoolID,
		SchoolName:       defaultSeedSchoolName,
		SchoolSlug:       defaultSeedSchoolSlug,
		TenantSlug:       tenantSlug,
		AdminEmail:       adminEmail,
		AdminPassword:    adminPassword,
		AdminName:        adminName,
		AdminPosition:    adminPosition,
	}, nil
}

func (s *Seeder) extractBootstrapInvitationToken(inviteResp []byte) (string, error) {
	var invitePayload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := parseJSON(inviteResp, &invitePayload); err == nil && invitePayload.Data.Token != "" {
		return invitePayload.Data.Token, nil
	}
	return "", fmt.Errorf("bootstrap invite token not available in operator response")
}

func (s *Seeder) createSeedOrganization(name, slug string) (int64, error) {
	orgResp, err := s.client.Post("/operator/organizations", map[string]any{
		"name": name,
		"slug": slug,
	})
	if err != nil {
		return 0, fmt.Errorf("create organization: %w", err)
	}
	var payload struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := parseJSON(orgResp, &payload); err != nil {
		return 0, fmt.Errorf("parse organization response: %w", err)
	}
	return payload.Data.ID, nil
}

func (s *Seeder) createSeedSchool(organizationID int64, name, slug, subdomain string) (int64, string, error) {
	schoolResp, err := s.client.Post("/operator/schools", map[string]any{
		"organization_id": organizationID,
		"name":            name,
		"slug":            slug,
		"subdomain":       subdomain,
	})
	if err != nil {
		return 0, "", fmt.Errorf("create school: %w", err)
	}
	var payload struct {
		Data struct {
			ID        int64  `json:"id"`
			Subdomain string `json:"subdomain"`
		} `json:"data"`
	}
	if err := parseJSON(schoolResp, &payload); err != nil {
		return 0, "", fmt.Errorf("parse school response: %w", err)
	}
	return payload.Data.ID, payload.Data.Subdomain, nil
}

// wrapConflictError checks if the error is a 409 Conflict from the API and
// returns a user-friendly message directing the developer to run migrate reset.
func wrapConflictError(err error, entity string) error {
	var apiErr *phoenixapi.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
		return fmt.Errorf("seed %s already exists (409 Conflict) — run 'go run main.go migrate reset' before re-seeding", entity)
	}
	return err
}

// collectSeedState builds SeedState from the FixedSeeder's internal maps
func (s *Seeder) collectSeedState(fs *FixedSeeder, staffPIN string, bootstrap *bootstrapSeedState) *SeedState {
	state := &SeedState{
		Version:    CurrentSeedStateVersion,
		CreatedAt:  time.Now().UTC(),
		BaseURL:    s.client.baseURL,
		DevicePIN:  staffPIN,
		Devices:    make(map[string]SeedDevice),
		Rooms:      make(map[string]int64),
		Activities: make(map[string]int64),
		Groups:     make(map[string]int64),
	}
	state.Bootstrap = makeBootstrapSeedState(bootstrap)

	s.populateSeedAccounts(state, fs)
	s.populateSeedDevices(state, fs)
	s.populateSeedStudents(state, fs)
	copySeedIDMap(state.Rooms, fs.roomIDs)
	copySeedIDMap(state.Activities, fs.activityIDs)
	copySeedIDMap(state.Groups, fs.groupIDs)
	state.Normalize()

	return state
}

func makeBootstrapSeedState(bootstrap *bootstrapSeedState) SeedStateBootstrap {
	if bootstrap == nil {
		return SeedStateBootstrap{}
	}
	return SeedStateBootstrap{
		OrganizationID:   bootstrap.OrganizationID,
		OrganizationName: bootstrap.OrganizationName,
		OrganizationSlug: bootstrap.OrganizationSlug,
		SchoolID:         bootstrap.SchoolID,
		SchoolName:       bootstrap.SchoolName,
		SchoolSlug:       bootstrap.SchoolSlug,
		TenantSlug:       bootstrap.TenantSlug,
		SchoolAdmin: BootstrapAdminCredentials{
			Email:    bootstrap.AdminEmail,
			Password: bootstrap.AdminPassword,
			Name:     bootstrap.AdminName,
			Position: bootstrap.AdminPosition,
		},
	}
}

func (s *Seeder) populateSeedAccounts(state *SeedState, fs *FixedSeeder) {
	groupKeys := []string{
		"sternengruppe", "bärengruppe", "sonnengruppe", "mondgruppe",
		"regenbogengruppe", "blumengruppe", "schmetterlingsgruppe",
		"waldgruppe", "meeresgruppe", "wiesengruppe",
	}
	betreuerIndex := 0
	for _, cred := range fs.staffCredentials {
		staffKey := cred.Name
		ac := AccountCredentials{
			Email:    cred.Email,
			Password: cred.Password,
			PIN:      cred.PIN,
			Name:     cred.Name,
			StaffID:  fs.staffIDs[staffKey],
		}
		if tid, ok := fs.teacherIDs[staffKey]; ok {
			ac.TeacherID = tid
		}

		switch cred.Position {
		case "OGS-Büro":
			state.Accounts.Admin = append(state.Accounts.Admin, ac)
		case "Pädagogische Fachkraft":
			if betreuerIndex < len(groupKeys) {
				ac.Group = groupKeys[betreuerIndex]
			}
			state.Accounts.Betreuer = append(state.Accounts.Betreuer, ac)
			betreuerIndex++
		}
	}
}

func (s *Seeder) populateSeedDevices(state *SeedState, fs *FixedSeeder) {
	for _, device := range DemoDevices {
		if apiKey, ok := fs.deviceKeys[device.DeviceID]; ok {
			state.Devices[device.DeviceID] = SeedDevice{
				APIKey: apiKey,
				Name:   device.Name,
			}
		}
	}
}

func (s *Seeder) populateSeedStudents(state *SeedState, fs *FixedSeeder) {
	for i, student := range DemoStudents {
		if id, ok := fs.studentIDByIndex[i]; ok {
			state.Students = append(state.Students, SeedStudent{
				ID:        id,
				FirstName: student.FirstName,
				LastName:  student.LastName,
				GroupKey:  student.GroupKey,
				Class:     student.Class,
			})
		}
	}
}

func copySeedIDMap(dst map[string]int64, src map[string]int64) {
	for key, id := range src {
		dst[key] = id
	}
}

// formatError creates a user-friendly error message
func (s *Seeder) formatError(stage string, err error) error {
	fmt.Printf("\nFailed at: %s\n", stage)
	fmt.Printf("  Error: %v\n\n", err)
	fmt.Println("Run './main migrate reset' and try again.")
	return fmt.Errorf("%s failed: %w", stage, err)
}

// printSuccessSummary prints the final demo-ready status with all created data
func (s *Seeder) printSuccessSummary(email, adminPassword string, result *SeedResult) {
	fmt.Println()
	fmt.Println("=== DEMO READY ===")
	fmt.Println()

	// Admin account used for seeding
	fmt.Println("ADMIN ACCOUNT (used for seeding):")
	fmt.Printf("  Email:    %s\n", email)
	fmt.Printf("  Password: %s\n", adminPassword)
	fmt.Println()

	// Staff accounts with correct individual passwords
	fmt.Println("STAFF ACCOUNTS:")
	fmt.Println("  Name                 | Position                | Email              | Password   | PIN")
	fmt.Println("  " + "--------------------" + " | " + "-----------------------" + " | " + "------------------" + " | " + "----------" + " | " + "----")
	for _, cred := range result.Fixed.StaffCredentials {
		fmt.Printf("  %-20s | %-23s | %-18s | %-10s | %s\n",
			cred.Name, cred.Position, cred.Email, cred.Password, cred.PIN)
	}
	fmt.Println()

	// Statistics
	fmt.Println("CREATED DATA:")
	fmt.Printf("  Rooms:            %d\n", result.Fixed.RoomCount)
	fmt.Printf("  Staff:            %d\n", result.Fixed.StaffCount)
	fmt.Printf("  Accounts:         %d\n", result.Fixed.AccountCount)
	fmt.Printf("  Groups:           %d\n", result.Fixed.GroupCount)
	fmt.Printf("  Students:         %d\n", result.Fixed.StudentCount)
	fmt.Printf("  Sick:             %d\n", result.Fixed.SickStudentCount)
	fmt.Printf("  Guardians:        %d\n", result.Fixed.GuardianCount)
	fmt.Printf("  Pickup schedules: %d\n", result.Fixed.PickupScheduleCount)
	fmt.Printf("  Activities:       %d\n", result.Fixed.ActivityCount)
	fmt.Printf("  IoT Devices:      %d\n", result.Fixed.DeviceCount)
	fmt.Println()

	fmt.Println("OUTPUT FILES:")
	fmt.Printf("  %s   (seed state with credentials & IDs)\n", DefaultSeedStatePath)
	fmt.Println("  simulator.yaml  (simulator configuration)")
	fmt.Println()
}
