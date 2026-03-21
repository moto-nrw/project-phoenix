package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

const (
	defaultSeedOrganizationName = "Demo Organization"
	defaultSeedOrganizationSlug = "demo-organization"
	defaultSeedSchoolName       = "Demo School"
	defaultSeedSchoolSlug       = "demo-school"
	defaultSeedSchoolSubdomain  = "demo-school"
	seedTokenHeader             = "X-Phoenix-Seed-Token"
	seedPasswordAlphabet        = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%"
	seedPasswordLength          = 12
)

// Seeder orchestrates the complete API-based seeding process
type Seeder struct {
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
	return &Seeder{
		client:  NewClient(baseURL, verbose),
		verbose: verbose,
	}
}

// Seed executes the complete seeding workflow
func (s *Seeder) Seed(ctx context.Context, email, password, staffPIN string) (*SeedResult, error) {
	result := &SeedResult{}

	// 1. Check server health
	fmt.Printf("Connecting to %s...\n", s.client.baseURL)
	if err := s.client.CheckHealth(); err != nil {
		return nil, s.formatError("Server health check", err)
	}

	// 2. Authenticate as operator
	fmt.Printf("Logging in as operator %s...\n", email)
	if err := s.client.LoginOperator(email, password); err != nil {
		return nil, s.formatError("Login", err)
	}
	fmt.Println("Operator authenticated")
	fmt.Println()

	bootstrapState, err := s.bootstrapTenant(ctx)
	if err != nil {
		return nil, s.formatError("Tenant bootstrap", err)
	}

	fmt.Printf("Logging in as invited school admin %s...\n", bootstrapState.AdminEmail)
	if err := s.client.Login(bootstrapState.AdminEmail, bootstrapState.AdminPassword, bootstrapState.TenantSlug); err != nil {
		return nil, s.formatError("Login", err)
	}
	fmt.Println("School admin authenticated")
	fmt.Println()

	// 3. Create Stammdaten (fixed/master data)
	fixedSeeder := NewFixedSeeder(s.client, s.verbose)
	fixedResult, err := fixedSeeder.Seed(ctx)
	if err != nil {
		return nil, s.formatError("Stammdaten seeding", err)
	}
	result.Fixed = fixedResult
	fmt.Println()

	// 4. Mark some students as sick
	if err := fixedSeeder.MarkStudentsSick(ctx, fixedResult); err != nil {
		return nil, s.formatError("Marking students sick", err)
	}

	// 5. Collect seed state and write .seed-state.json
	state := s.collectSeedState(fixedSeeder, staffPIN, bootstrapState)
	statePath := DefaultSeedStatePath
	if err := WriteSeedState(state, statePath); err != nil {
		return nil, s.formatError("Writing seed state", err)
	}
	fmt.Printf("Seed state written to %s\n", statePath)

	// 6. Generate simulator.yaml
	simPath := "simulator/iot/simulator.yaml"
	if err := WriteSimulatorConfig(state, simPath); err != nil {
		return nil, s.formatError("Generating simulator config", err)
	}
	fmt.Printf("Simulator config written to %s\n", simPath)
	fmt.Println()

	// 7. Print success summary
	s.printSuccessSummary(bootstrapState.AdminEmail, bootstrapState.AdminPassword, result)

	return result, nil
}

func (s *Seeder) bootstrapTenant(ctx context.Context) (*bootstrapSeedState, error) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	orgName := fmt.Sprintf("%s %s", defaultSeedOrganizationName, suffix)
	orgSlug := fmt.Sprintf("%s-%s", defaultSeedOrganizationSlug, suffix)
	orgID, err := s.createSeedOrganization(orgName, orgSlug)
	if err != nil {
		return nil, err
	}

	schoolName := fmt.Sprintf("%s %s", defaultSeedSchoolName, suffix)
	schoolSlug := fmt.Sprintf("%s-%s", defaultSeedSchoolSlug, suffix)
	schoolSubdomain := truncateSeedSubdomain(fmt.Sprintf("%s-%s", defaultSeedSchoolSubdomain, suffix))
	schoolID, tenantSlug, err := s.createSeedSchool(orgID, schoolName, schoolSlug, schoolSubdomain)
	if err != nil {
		return nil, err
	}

	adminEmail := fmt.Sprintf("school-admin-%s@example.com", suffix)
	adminPassword, err := generateSeedPassword()
	if err != nil {
		return nil, fmt.Errorf("generate admin password: %w", err)
	}
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
		OrganizationName: orgName,
		OrganizationSlug: orgSlug,
		SchoolID:         schoolID,
		SchoolName:       schoolName,
		SchoolSlug:       schoolSlug,
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

func generateSeedPassword() (string, error) {
	// Character sets that must each appear at least once to satisfy
	// ValidatePasswordStrength (upper, lower, digit, special).
	required := []string{
		"abcdefghijkmnopqrstuvwxyz",
		"ABCDEFGHJKLMNPQRSTUVWXYZ",
		"23456789",
		"!@#$%",
	}

	bytes := make([]byte, seedPasswordLength)
	random := make([]byte, seedPasswordLength)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	// Place one character from each required set in the first positions.
	for i, charset := range required {
		bytes[i] = charset[int(random[i])%len(charset)]
	}

	// Fill remaining positions from the full alphabet.
	for i := len(required); i < seedPasswordLength; i++ {
		bytes[i] = seedPasswordAlphabet[int(random[i])%len(seedPasswordAlphabet)]
	}

	// Shuffle using Fisher-Yates to avoid predictable positions.
	shuffleRandom := make([]byte, seedPasswordLength)
	if _, err := rand.Read(shuffleRandom); err != nil {
		return "", err
	}
	for i := seedPasswordLength - 1; i > 0; i-- {
		j := int(shuffleRandom[i]) % (i + 1)
		bytes[i], bytes[j] = bytes[j], bytes[i]
	}

	return string(bytes), nil
}

func truncateSeedSubdomain(subdomain string) string {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if len(subdomain) <= 63 {
		return subdomain
	}
	return subdomain[:63]
}

// collectSeedState builds SeedState from the FixedSeeder's internal maps
func (s *Seeder) collectSeedState(fs *FixedSeeder, staffPIN string, bootstrap *bootstrapSeedState) *SeedState {
	state := &SeedState{
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
	fmt.Printf("  Rooms:       %d\n", result.Fixed.RoomCount)
	fmt.Printf("  Staff:       %d\n", result.Fixed.StaffCount)
	fmt.Printf("  Accounts:    %d\n", result.Fixed.AccountCount)
	fmt.Printf("  Groups:      %d\n", result.Fixed.GroupCount)
	fmt.Printf("  Students:    %d\n", result.Fixed.StudentCount)
	fmt.Printf("  Sick:        %d\n", result.Fixed.SickStudentCount)
	fmt.Printf("  Guardians:   %d\n", result.Fixed.GuardianCount)
	fmt.Printf("  Activities:  %d\n", result.Fixed.ActivityCount)
	fmt.Printf("  IoT Devices: %d\n", result.Fixed.DeviceCount)
	fmt.Println()

	fmt.Println("OUTPUT FILES:")
	fmt.Printf("  %s   (seed state with credentials & IDs)\n", DefaultSeedStatePath)
	fmt.Println("  simulator.yaml  (simulator configuration)")
	fmt.Println()
}
