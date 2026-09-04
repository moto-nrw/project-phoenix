package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"
)

const (
	defaultSeedOrganizationName = "Demo Organization"
	defaultSeedOrganizationSlug = "demo-organization"
	defaultSeedSchoolName       = "Demo School"
	defaultSeedSchoolSubdomain  = "demo-school"
	seedTokenHeader             = "X-Phoenix-Seed-Token"
	defaultSeedParentPassword   = "ParentSeed1234%"
	seedPasswordAlphabet        = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%"
	seedPasswordLength          = 12
)

// SeedOptions controls the API seeder. The default is the stable vollbetrieb
// profile; Randomize is an explicit escape hatch for parallel ad-hoc stacks.
type SeedOptions struct {
	TenantSlug    string // Optional tenant slug override
	StaffPassword string // Shared password for all 20 staff accounts
	AdminEmail    string // Optional bootstrap school admin email override
	Randomize     bool   // Append unique suffixes and generate admin credentials
	StatePath     string // Output path; empty uses DefaultSeedStatePath
}

// Seeder orchestrates the complete API-based seeding process
type Seeder struct {
	client     *Client
	random     io.Reader
	verbose    bool
	options    SeedOptions
	statePath  string
	profile    string
	definition demoProfileDefinition
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

type seedProfileIdentity struct {
	organizationName string
	organizationSlug string
	schoolName       string
	schoolSlug       string
}

// NewSeeder creates a new API seeder
func NewSeeder(adapter Adapter, random io.Reader, verbose bool, options SeedOptions) *Seeder {
	statePath := options.StatePath
	if statePath == "" {
		statePath = DefaultSeedStatePath
	}
	return &Seeder{
		client:     NewClientWithAdapter(adapter, verbose),
		random:     random,
		verbose:    verbose,
		options:    options,
		statePath:  statePath,
		profile:    DefaultProfileKey,
		definition: fullOperationProfileDefinition(),
	}
}

// Seed executes the complete seeding workflow
func (s *Seeder) Seed(ctx context.Context, email, password, staffPIN string) (*SeedResult, error) {
	if s.client == nil || s.client.adapter == nil {
		return nil, fmt.Errorf("seed API adapter is required")
	}
	if s.random == nil {
		return nil, fmt.Errorf("seed random source is required")
	}
	runtime := newRuntime(s, email, password, staffPIN)
	workflow := fullDemoWorkflow(s)
	if err := workflow.Run(ctx, runtime); err != nil {
		var stepErr *StepError
		if errors.As(err, &stepErr) {
			return nil, s.formatProfileError(s.profile, stepErr.Step, stepErr.Err)
		}
		return nil, s.formatProfileError(s.profile, workflow.Name, err)
	}
	return runtime.Result, nil
}

// bootstrapTenant creates the demo organization, school, and admin account.
// Stable profile values are the default, so re-running without a database
// reset fails clearly with 409. Randomize must be requested explicitly.
func (s *Seeder) bootstrapTenant(ctx context.Context) (*bootstrapSeedState, error) {
	identity := s.profileIdentity()
	orgID, err := s.createSeedOrganization(identity.organizationName, identity.organizationSlug)
	if err != nil {
		return nil, wrapConflictError(err, "organization")
	}
	schoolID, tenantSlug, err := s.createSeedSchool(orgID, identity.schoolName, identity.schoolSlug, identity.schoolSlug)
	if err != nil {
		return nil, wrapConflictError(err, "school")
	}
	adminEmail, adminPassword, err := s.profileAdminCredentials()
	if err != nil {
		return nil, err
	}
	adminName := "Seed Admin"
	adminPosition := "OGS-Büro"
	if err := s.inviteSeedAdmin(schoolID, adminEmail, adminPassword, adminPosition); err != nil {
		return nil, err
	}
	return &bootstrapSeedState{
		OrganizationID: orgID, OrganizationName: identity.organizationName, OrganizationSlug: identity.organizationSlug,
		SchoolID: schoolID, SchoolName: identity.schoolName, SchoolSlug: identity.schoolSlug, TenantSlug: tenantSlug,
		AdminEmail: adminEmail, AdminPassword: adminPassword, AdminName: adminName, AdminPosition: adminPosition,
	}, nil
}

func (s *Seeder) profileIdentity() seedProfileIdentity {
	if s.options.Randomize {
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		return seedProfileIdentity{
			organizationName: fmt.Sprintf("%s %s", defaultSeedOrganizationName, suffix),
			organizationSlug: fmt.Sprintf("%s-%s", defaultSeedOrganizationSlug, suffix),
			schoolName:       fmt.Sprintf("%s %s", defaultSeedSchoolName, suffix),
			schoolSlug:       truncateSeedSubdomain(fmt.Sprintf("%s-%s", defaultSeedSchoolSubdomain, suffix)),
		}
	}
	slug := s.definition.SchoolSlug
	if s.options.TenantSlug != "" {
		slug = s.options.TenantSlug
	}
	return seedProfileIdentity{
		organizationName: s.definition.OrganizationName, organizationSlug: s.definition.OrganizationSlug,
		schoolName: s.definition.SchoolName, schoolSlug: slug,
	}
}

func (s *Seeder) profileAdminCredentials() (string, string, error) {
	email := s.definition.SchoolAdminEmail
	if s.options.AdminEmail != "" {
		email = s.options.AdminEmail
	} else if s.options.Randomize {
		email = fmt.Sprintf("school-admin-%d@example.com", time.Now().UnixNano())
	}
	if s.options.StaffPassword != "" {
		return email, s.options.StaffPassword, nil
	}
	if !s.options.Randomize {
		return email, s.definition.SchoolAdminPassword, nil
	}
	password, err := generateSeedPassword(s.random)
	if err != nil {
		return "", "", fmt.Errorf("generate admin password: %w", err)
	}
	return email, password, nil
}

func (s *Seeder) inviteSeedAdmin(schoolID int64, email, password, position string) error {
	inviteResp, err := s.client.PostWithHeaders(fmt.Sprintf("/operator/schools/%d/invite-admin", schoolID), map[string]any{
		"email":      email,
		"first_name": "Seed",
		"last_name":  "Admin",
		"position":   position,
	}, map[string]string{
		seedTokenHeader: "true",
	})
	if err != nil {
		return fmt.Errorf("invite school admin: %w", err)
	}
	token, err := s.extractBootstrapInvitationToken(inviteResp)
	if err != nil {
		return err
	}

	if _, err := s.client.PostPublic(fmt.Sprintf("/auth/invitations/%s/accept", token), map[string]any{
		"password":         password,
		"confirm_password": password,
	}); err != nil {
		return fmt.Errorf("accept invitation: %w", err)
	}
	return nil
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

func generateSeedPassword(randomSource io.Reader) (string, error) {
	if randomSource == nil {
		return "", fmt.Errorf("seed random source is required")
	}
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
	if _, err := io.ReadFull(randomSource, random); err != nil {
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
	if _, err := io.ReadFull(randomSource, shuffleRandom); err != nil {
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

// wrapConflictError checks if the error is a 409 Conflict from the API and
// returns a user-friendly message directing the developer to run migrate reset.
func wrapConflictError(err error, entity string) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
		return fmt.Errorf("seed %s already exists (409 Conflict) — run 'docker compose run server go run . migrate reset' before re-seeding", entity)
	}
	return err
}

// collectSeedState builds SeedState from the FixedSeeder's internal maps
func (s *Seeder) collectSeedState(fs *FixedSeeder, staffPIN string, bootstrap *bootstrapSeedState) *SeedState {
	state := &SeedState{
		Version:        CurrentSeedStateVersion,
		CreatedAt:      time.Now().UTC(),
		BaseURL:        s.client.baseURL,
		DefaultProfile: s.definition.Key,
		DevicePIN:      staffPIN,
		Devices:        make(map[string]SeedDevice),
		Rooms:          make(map[string]int64),
		Activities:     make(map[string]int64),
		Groups:         make(map[string]int64),
		Settings:       cloneProfileSettings(s.definition.Settings),
		Expected:       s.definition.Expected,
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
				DeviceID:   device.DeviceID,
				DeviceType: "terminal",
				APIKey:     apiKey,
				Name:       device.Name,
			}
		}
	}
}

func (s *Seeder) populateSeedStudents(state *SeedState, fs *FixedSeeder) {
	for i, student := range DemoStudents {
		if id, ok := fs.studentIDByIndex[i]; ok {
			state.Students = append(state.Students, SeedStudent{
				Key:       semanticKey(student.FirstName + " " + student.LastName),
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

func (s *Seeder) formatProfileError(profile, stage string, err error) error {
	fmt.Printf("\nFailed at: %s\n", stage)
	fmt.Printf("  Error: %v\n\n", err)
	request := ""
	if s.client != nil && s.client.lastMethod != "" && !strings.Contains(err.Error(), s.client.lastMethod+" "+s.client.lastPath) {
		request = fmt.Sprintf(", last request %s %s returned status %d", s.client.lastMethod, s.client.lastPath, s.client.lastStatus)
	}
	return fmt.Errorf("demo school profile %q, workflow step %s failed%s: %w", profile, stage, request, err)
}

// printSuccessSummary prints the final demo-ready status with all created data
func (s *Seeder) printSuccessSummary(email, adminPassword string, result *SeedResult, state *SeedState) {
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

	if state != nil && len(state.Parents) > 0 {
		fmt.Println("PARENT ACCOUNTS:")
		fmt.Println("  Name                 | Email                         | Password     | Student IDs")
		fmt.Println("  " + "--------------------" + " | " + "-----------------------------" + " | " + "------------" + " | " + "-----------")
		for _, parent := range state.Parents {
			fmt.Printf("  %-20s | %-29s | %-12s | %s\n",
				parent.Name, parent.Email, parent.Password, formatSeedIDList(parent.StudentIDs))
		}
		fmt.Println()
	}
	printCareWithdrawalDemo(state)

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
	fmt.Printf("  Class arrival times: %d\n", result.Fixed.ClassArrivalTimeCount)
	fmt.Printf("  Class arrival exceptions: %d\n", result.Fixed.ClassArrivalExceptionCount)
	fmt.Printf("  Activities:       %d\n", result.Fixed.ActivityCount)
	fmt.Printf("  IoT Devices:      %d\n", result.Fixed.DeviceCount)
	if state != nil {
		fmt.Printf("  Parent accounts:  %d\n", len(state.Parents))
		if state.Enrollment.PhaseID != 0 {
			fmt.Printf("  Enrollment phase: %d\n", state.Enrollment.PhaseID)
		}
		if len(state.Enrollment.Offerings) > 0 {
			fmt.Printf("  Care offerings:   %d (%s)\n", len(state.Enrollment.Offerings), formatSortedCountMap(state.Enrollment.Offerings))
		}
		if len(state.Enrollment.Requests) > 0 {
			fmt.Printf("  Enroll. requests: %d (%s)\n", len(state.Enrollment.Requests), formatEnrollmentStatusCounts(state.Enrollment.Requests))
		}
		if len(state.Enrollment.ParentActions) > 0 {
			fmt.Printf("  Parent actions:   %d (%s)\n", len(state.Enrollment.ParentActions), formatParentActionCounts(state.Enrollment.ParentActions))
		}
	}
	fmt.Println()

	fmt.Println("OUTPUT FILES:")
	fmt.Printf("  %s   (seed state with credentials & IDs)\n", s.statePath)
	fmt.Println()
}

func printCareWithdrawalDemo(state *SeedState) {
	if state == nil || state.CareWithdrawals == nil {
		return
	}
	demo := state.CareWithdrawals
	fmt.Println("CARE WITHDRAWAL DEMO:")
	fmt.Printf("  School:   %s (%s)\n", demo.SchoolName, demo.TenantSlug)
	fmt.Printf("  Email:    %s\n", demo.SchoolAdmin.Email)
	fmt.Printf("  Password: %s\n", demo.SchoolAdmin.Password)
	fmt.Println()
}

func formatSeedIDList(ids []int64) string {
	if len(ids) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf("%d", id))
	}
	return strings.Join(parts, ", ")
}

func formatEnrollmentStatusCounts(requests []SeedEnrollmentRequest) string {
	return formatSortedCountMap(countBy(requests, func(request SeedEnrollmentRequest) string {
		if status := strings.TrimSpace(request.Status); status != "" {
			return status
		}
		return "submitted"
	}))
}

func formatParentActionCounts(actions []SeedParentPortalAction) string {
	return formatSortedCountMap(countBy(actions, func(action SeedParentPortalAction) string {
		if actionType := strings.TrimSpace(action.Type); actionType != "" {
			return actionType
		}
		return "unknown"
	}))
}

// formatSortedCountMap renders a string->number map as "key=value" pairs sorted
// by key, or "-" when empty.
func formatSortedCountMap[V int | int64](values map[string]V) string {
	if len(values) == 0 {
		return "-"
	}
	keys := slices.Sorted(maps.Keys(values))
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

// countBy tallies items by the string key derived from each element.
func countBy[T any](items []T, key func(T) string) map[string]int {
	counts := make(map[string]int)
	for _, item := range items {
		counts[key(item)]++
	}
	return counts
}
