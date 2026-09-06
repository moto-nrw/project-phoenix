package demoprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
)

const DefaultSeedStatePath = ".seed-state.json"
const CurrentSeedStateVersion = "3"
const DefaultProfileKey = "vollbetrieb"
const ManualProfileKey = "manuell"

const (
	SettingManagedByOperator = "operator"
	SettingManagedByTenant   = "tenant"
)

// SeedState is the versioned contract shared by the seeder, simulator, and
// downstream test tools. Version 3 stores schools by semantic profile key.
// The json:"-" fields are an in-memory view used by the existing seeder; they
// are projected into Profiles by Normalize and never recreate the old flat
// wire format.
type SeedState struct {
	Version        string                      `json:"version"`
	CreatedAt      time.Time                   `json:"created_at"`
	BaseURL        string                      `json:"base_url"`
	DefaultProfile string                      `json:"default_profile"`
	Organizations  map[string]SeedOrganization `json:"organizations"`
	Profiles       map[string]*SeedProfile     `json:"profiles"`
	Topology       SeedStateTopology           `json:"topology"`

	DevicePIN   string                 `json:"-"`
	Bootstrap   SeedStateBootstrap     `json:"-"`
	Accounts    SeedStateAccounts      `json:"-"`
	Parents     []ParentCredentials    `json:"-"`
	Devices     map[string]SeedDevice  `json:"-"`
	Students    []SeedStudent          `json:"-"`
	Rooms       map[string]int64       `json:"-"`
	Activities  map[string]int64       `json:"-"`
	Groups      map[string]int64       `json:"-"`
	Enrollment  SeedEnrollmentState    `json:"-"`
	Credentials SeedStateCredentials   `json:"-"`
	Entities    SeedStateEntities      `json:"-"`
	Lookups     SeedStateLookups       `json:"-"`
	Scenarios   SeedStateScenarios     `json:"-"`
	Settings    map[string]SeedSetting `json:"-"`
	Expected    SeedExpectedState      `json:"-"`
}

type SeedOrganization struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Slug     string   `json:"slug"`
	Profiles []string `json:"profiles"`
}

type SeedProfile struct {
	Key          string                 `json:"key"`
	Name         string                 `json:"name"`
	Organization SeedOrganizationRef    `json:"organization"`
	School       SeedSchoolRef          `json:"school"`
	Settings     map[string]SeedSetting `json:"settings"`
	Credentials  SeedStateCredentials   `json:"credentials"`
	Devices      map[string]SeedDevice  `json:"devices"`
	Entities     SeedProfileEntities    `json:"entities"`
	Expected     SeedExpectedState      `json:"expected"`
	Scenarios    SeedStateScenarios     `json:"scenarios"`
}

type SeedOrganizationRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type SeedSchoolRef struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	TenantSlug string `json:"tenant_slug"`
}

type SeedSetting struct {
	Value     json.RawMessage `json:"value"`
	ManagedBy string          `json:"managed_by"`
}

type SeedProfileEntities struct {
	Students   map[string]SeedStudent   `json:"students"`
	Guardians  map[string]SeedEntityRef `json:"guardians,omitempty"`
	Rooms      map[string]SeedEntityRef `json:"rooms"`
	Activities map[string]SeedEntityRef `json:"activities"`
	Groups     map[string]SeedEntityRef `json:"groups"`
	Enrollment SeedEnrollmentState      `json:"enrollment,omitempty"`
}

type SeedEntityRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SeedExpectedState struct {
	Students            int                        `json:"students"`
	Rooms               int                        `json:"rooms"`
	Groups              int                        `json:"groups"`
	Staff               int                        `json:"staff"`
	Contacts            int                        `json:"contacts"`
	ParentAccounts      int                        `json:"parent_accounts"`
	PhysicalDevices     int                        `json:"physical_devices"`
	HasEnrollment       bool                       `json:"has_enrollment"`
	HasAttendance       bool                       `json:"has_attendance"`
	HasHistory          bool                       `json:"has_history"`
	PresentStudents     int                        `json:"present_students,omitempty"`
	CheckedOutStudents  int                        `json:"checked_out_students,omitempty"`
	WeeklyPlans         int                        `json:"weekly_plans,omitempty"`
	ScheduledActivities SeedScheduledActivityState `json:"scheduled_activities"`
}

type SeedScheduledActivityState struct {
	Minimum  int  `json:"minimum"`
	Rooms    bool `json:"rooms"`
	Students bool `json:"students"`
	Staff    bool `json:"staff"`
}

type SeedStateCredentials struct {
	Operator    *SeedOperatorCredentials  `json:"operator,omitempty"`
	SchoolAdmin BootstrapAdminCredentials `json:"school_admin"`
	DevicePIN   string                    `json:"device_pin"`
	Accounts    SeedStateAccounts         `json:"accounts"`
	Parents     []ParentCredentials       `json:"parents,omitempty"`
}

type SeedOperatorCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SeedStateTopology struct {
	Organizations int    `json:"organizations"`
	Schools       int    `json:"schools"`
	Mode          string `json:"mode,omitempty"`
}

// SeedStateEntities and SeedStateLookups remain as the seeder's in-memory
// view. SeedProfileEntities is the v3 wire contract.
type SeedStateEntities struct {
	Bootstrap  SeedStateBootstrap    `json:"bootstrap"`
	Devices    map[string]SeedDevice `json:"devices"`
	Students   []SeedStudent         `json:"students"`
	Enrollment SeedEnrollmentState   `json:"enrollment,omitempty"`
}

type SeedStateLookups struct {
	Rooms      map[string]int64 `json:"rooms"`
	Activities map[string]int64 `json:"activities"`
	Groups     map[string]int64 `json:"groups"`
}

type SeedStateScenarios struct {
	DefaultPlayer string `json:"default_player,omitempty"`
	DefaultMode   string `json:"default_mode,omitempty"`
}

type SeedStateBootstrap struct {
	OrganizationID   int64                     `json:"organization_id"`
	OrganizationName string                    `json:"organization_name"`
	OrganizationSlug string                    `json:"organization_slug"`
	SchoolID         int64                     `json:"school_id"`
	SchoolName       string                    `json:"school_name"`
	SchoolSlug       string                    `json:"school_slug"`
	TenantSlug       string                    `json:"tenant_slug"`
	SchoolAdmin      BootstrapAdminCredentials `json:"school_admin"`
}

type SeedStateAccounts struct {
	Admin    []AccountCredentials `json:"admin"`
	Betreuer []AccountCredentials `json:"betreuer"`
}

type ParentCredentials struct {
	Key        string  `json:"key"`
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Name       string  `json:"name"`
	AccountID  int64   `json:"account_id,omitempty"`
	GuardianID int64   `json:"guardian_id"`
	StudentIDs []int64 `json:"student_ids,omitempty"`
}

type BootstrapAdminCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

type AccountCredentials struct {
	Key       string `json:"key"`
	AccountID int64  `json:"account_id,omitempty"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	PIN       string `json:"pin"`
	Name      string `json:"name"`
	StaffID   int64  `json:"staff_id"`
	TeacherID int64  `json:"teacher_id,omitempty"`
	Group     string `json:"group,omitempty"`
}

type SeedDevice struct {
	ID         int64  `json:"id,omitempty"`
	DeviceID   string `json:"device_id"`
	DeviceType string `json:"device_type"`
	APIKey     string `json:"api_key,omitempty"`
	Name       string `json:"name"`
	Protected  bool   `json:"protected"`
}

type SeedStudent struct {
	Key       string `json:"key"`
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	GroupKey  string `json:"group_key"`
	Class     string `json:"class"`
}

type SeedEnrollmentState struct {
	SchemaID      int64                      `json:"schema_id,omitempty"`
	PhaseID       int64                      `json:"phase_id,omitempty"`
	Offerings     map[string]int64           `json:"offerings,omitempty"`
	Requests      []SeedEnrollmentRequest    `json:"requests,omitempty"`
	ParentActions []SeedParentPortalAction   `json:"parent_actions,omitempty"`
	Settings      map[string]json.RawMessage `json:"settings,omitempty"`
}

type SeedEnrollmentRequest struct {
	Key         string  `json:"key,omitempty"`
	RequestID   int64   `json:"request_id"`
	Source      string  `json:"source"`
	Status      string  `json:"status,omitempty"`
	StatusURL   string  `json:"status_url,omitempty"`
	StatusToken string  `json:"status_token,omitempty"`
	ChildIDs    []int64 `json:"child_ids,omitempty"`
}

type SeedParentPortalAction struct {
	ParentEmail string `json:"parent_email"`
	StudentID   int64  `json:"student_id"`
	Type        string `json:"type"`
}

func WriteSeedState(state *SeedState, path string) error {
	if state == nil {
		return fmt.Errorf("seed state is required")
	}
	if state.Version != "" && state.Version != CurrentSeedStateVersion {
		return fmt.Errorf("cannot write unsupported seed state version %q", state.Version)
	}
	state.Normalize()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seed state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write seed state: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("restrict seed state permissions: %w", err)
	}
	return nil
}

func LoadSeedState(path string) (*SeedState, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read seed state: %w", err)
	}
	var header struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("unmarshal seed state: %w", err)
	}
	if header.Version == "" || header.Version == "2" {
		return loadLegacySeedState(data)
	}
	if header.Version != CurrentSeedStateVersion {
		return nil, fmt.Errorf("unsupported seed state version %q; supported version is %q", header.Version, CurrentSeedStateVersion)
	}

	var state SeedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal seed state: %w", err)
	}
	if state.DefaultProfile == "" {
		return nil, fmt.Errorf("seed state version %s has no default_profile", CurrentSeedStateVersion)
	}
	if _, ok := state.Profiles[state.DefaultProfile]; !ok {
		return nil, fmt.Errorf("seed state default profile %q is missing", state.DefaultProfile)
	}
	state.hydrateProfile(state.DefaultProfile)
	return &state, nil
}

func (s *SeedState) SelectProfile(key string) (*SeedProfile, error) {
	if s == nil {
		return nil, fmt.Errorf("seed state is required")
	}
	if key == "" {
		key = s.DefaultProfile
	}
	profile, ok := s.Profiles[key]
	if !ok || profile == nil {
		keys := make([]string, 0, len(s.Profiles))
		for available := range s.Profiles {
			keys = append(keys, available)
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("unknown demo school profile %q; available profiles: %s", key, strings.Join(keys, ", "))
	}
	return profile, nil
}

func (s *SeedState) Normalize() {
	if s == nil {
		return
	}
	if s.Version == "" {
		s.Version = CurrentSeedStateVersion
	}
	if s.DefaultProfile == "" {
		s.DefaultProfile = DefaultProfileKey
	}
	normalizeFlatState(s)
	if hasFlatProfile(s) || len(s.Profiles) == 0 {
		s.syncProfile(s.DefaultProfile)
	} else if len(s.Profiles) > 0 {
		s.hydrateProfile(s.DefaultProfile)
	}
	if s.Topology.Organizations == 0 {
		s.Topology.Organizations = len(s.Organizations)
	}
	if s.Topology.Schools == 0 {
		s.Topology.Schools = len(s.Profiles)
	}
	if s.Topology.Mode == "" {
		s.Topology.Mode = "profiles"
	}
}

func normalizeFlatState(s *SeedState) {
	normalizeFlatCollections(s)
	normalizeFlatCredentials(s)
	if s.Scenarios.DefaultPlayer == "" {
		s.Scenarios.DefaultPlayer = "pyreportal"
	}
	if s.Scenarios.DefaultMode == "" {
		s.Scenarios.DefaultMode = "hybrid"
	}
	normalizeEnrollmentState(&s.Enrollment)
	normalizeSemanticKeys(s)
}

func normalizeFlatCollections(s *SeedState) {
	if s.Devices == nil {
		s.Devices = make(map[string]SeedDevice)
	}
	if s.Rooms == nil {
		s.Rooms = make(map[string]int64)
	}
	if s.Activities == nil {
		s.Activities = make(map[string]int64)
	}
	if s.Groups == nil {
		s.Groups = make(map[string]int64)
	}
	if s.Settings == nil {
		s.Settings = make(map[string]SeedSetting)
	}
	if s.Organizations == nil {
		s.Organizations = make(map[string]SeedOrganization)
	}
	if s.Profiles == nil {
		s.Profiles = make(map[string]*SeedProfile)
	}
}

func normalizeFlatCredentials(s *SeedState) {
	if s.Credentials.DevicePIN == "" {
		s.Credentials.DevicePIN = s.DevicePIN
	}
	if s.DevicePIN == "" {
		s.DevicePIN = s.Credentials.DevicePIN
	}
	if len(s.Credentials.Accounts.Admin) == 0 && len(s.Credentials.Accounts.Betreuer) == 0 {
		s.Credentials.Accounts = s.Accounts
	}
	if len(s.Accounts.Admin) == 0 && len(s.Accounts.Betreuer) == 0 {
		s.Accounts = s.Credentials.Accounts
	}
	if len(s.Credentials.Parents) == 0 {
		s.Credentials.Parents = append([]ParentCredentials(nil), s.Parents...)
	}
	if len(s.Parents) == 0 {
		s.Parents = append([]ParentCredentials(nil), s.Credentials.Parents...)
	}
	if s.Credentials.SchoolAdmin.Email == "" {
		s.Credentials.SchoolAdmin = s.Bootstrap.SchoolAdmin
	}
	if s.Bootstrap.SchoolAdmin.Email == "" {
		s.Bootstrap.SchoolAdmin = s.Credentials.SchoolAdmin
	}
}

func normalizeSemanticKeys(s *SeedState) {
	for i := range s.Accounts.Admin {
		if s.Accounts.Admin[i].Key == "" {
			s.Accounts.Admin[i].Key = SemanticKey(s.Accounts.Admin[i].Name)
		}
	}
	for i := range s.Accounts.Betreuer {
		if s.Accounts.Betreuer[i].Key == "" {
			s.Accounts.Betreuer[i].Key = SemanticKey(s.Accounts.Betreuer[i].Name)
		}
	}
	for i := range s.Parents {
		if s.Parents[i].Key == "" {
			s.Parents[i].Key = SemanticKey(s.Parents[i].Name)
		}
	}
	for i := range s.Students {
		if s.Students[i].Key == "" {
			s.Students[i].Key = SemanticKey(s.Students[i].FirstName + " " + s.Students[i].LastName)
		}
	}
}

func hasFlatProfile(s *SeedState) bool {
	return s.Bootstrap.SchoolID != 0 || len(s.Accounts.Admin)+len(s.Accounts.Betreuer) > 0 ||
		len(s.Devices)+len(s.Students)+len(s.Rooms)+len(s.Activities)+len(s.Groups)+len(s.Settings) > 0
}

func (s *SeedState) syncProfile(key string) {
	profile := &SeedProfile{
		Key:  key,
		Name: s.Bootstrap.SchoolName,
		Organization: SeedOrganizationRef{
			ID: s.Bootstrap.OrganizationID, Name: s.Bootstrap.OrganizationName, Slug: s.Bootstrap.OrganizationSlug,
		},
		School: SeedSchoolRef{
			ID: s.Bootstrap.SchoolID, Name: s.Bootstrap.SchoolName, Slug: s.Bootstrap.SchoolSlug, TenantSlug: s.Bootstrap.TenantSlug,
		},
		Settings:    cloneSeedSettings(s.Settings),
		Credentials: s.Credentials,
		Devices:     cloneSeedDevices(s.Devices),
		Entities:    profileEntities(s),
		Expected:    s.Expected,
		Scenarios:   s.Scenarios,
	}
	profile.Credentials.DevicePIN = s.DevicePIN
	profile.Credentials.SchoolAdmin = s.Bootstrap.SchoolAdmin
	profile.Credentials.Accounts = s.Accounts
	profile.Credentials.Parents = append([]ParentCredentials(nil), s.Parents...)
	s.Profiles[key] = profile
	if profile.Organization.Slug != "" {
		profileKeys := []string{key}
		if existing, ok := s.Organizations[profile.Organization.Slug]; ok {
			profileKeys = append(profileKeys, existing.Profiles...)
		}
		sort.Strings(profileKeys)
		profileKeys = slices.Compact(profileKeys)
		s.Organizations[profile.Organization.Slug] = SeedOrganization{
			ID: profile.Organization.ID, Name: profile.Organization.Name, Slug: profile.Organization.Slug, Profiles: profileKeys,
		}
	}
}

func profileEntities(s *SeedState) SeedProfileEntities {
	entities := SeedProfileEntities{
		Students:   make(map[string]SeedStudent, len(s.Students)),
		Rooms:      seedEntityRefs(s.Rooms),
		Activities: seedEntityRefs(s.Activities),
		Groups:     seedEntityRefs(s.Groups),
		Enrollment: CloneEnrollmentState(s.Enrollment),
	}
	for _, student := range s.Students {
		entities.Students[student.Key] = student
	}
	return entities
}

func seedEntityRefs(values map[string]int64) map[string]SeedEntityRef {
	refs := make(map[string]SeedEntityRef, len(values))
	for name, id := range values {
		refs[SemanticKey(name)] = SeedEntityRef{ID: id, Name: name}
	}
	return refs
}

func (s *SeedState) hydrateProfile(key string) {
	profile, ok := s.Profiles[key]
	if !ok || profile == nil {
		return
	}
	s.Bootstrap = SeedStateBootstrap{
		OrganizationID: profile.Organization.ID, OrganizationName: profile.Organization.Name, OrganizationSlug: profile.Organization.Slug,
		SchoolID: profile.School.ID, SchoolName: profile.School.Name, SchoolSlug: profile.School.Slug,
		TenantSlug: profile.School.TenantSlug, SchoolAdmin: profile.Credentials.SchoolAdmin,
	}
	s.Settings = cloneSeedSettings(profile.Settings)
	s.Credentials = profile.Credentials
	s.DevicePIN = profile.Credentials.DevicePIN
	s.Accounts = profile.Credentials.Accounts
	s.Parents = append([]ParentCredentials(nil), profile.Credentials.Parents...)
	s.Devices = cloneSeedDevices(profile.Devices)
	s.Students = make([]SeedStudent, 0, len(profile.Entities.Students))
	studentKeys := make([]string, 0, len(profile.Entities.Students))
	for studentKey := range profile.Entities.Students {
		studentKeys = append(studentKeys, studentKey)
	}
	sort.Strings(studentKeys)
	for _, studentKey := range studentKeys {
		s.Students = append(s.Students, profile.Entities.Students[studentKey])
	}
	s.Rooms = flattenSeedEntityRefs(profile.Entities.Rooms)
	s.Activities = flattenSeedEntityRefs(profile.Entities.Activities)
	s.Groups = flattenSeedEntityRefs(profile.Entities.Groups)
	s.Enrollment = CloneEnrollmentState(profile.Entities.Enrollment)
	s.Scenarios = profile.Scenarios
	s.Expected = profile.Expected
}

func flattenSeedEntityRefs(values map[string]SeedEntityRef) map[string]int64 {
	flat := make(map[string]int64, len(values))
	for key, ref := range values {
		name := ref.Name
		if name == "" {
			name = key
		}
		flat[name] = ref.ID
	}
	return flat
}

func cloneSeedDevices(src map[string]SeedDevice) map[string]SeedDevice {
	dst := make(map[string]SeedDevice, len(src))
	for key, value := range src {
		if value.DeviceID == "" {
			value.DeviceID = key
		}
		if value.DeviceType == "" {
			value.DeviceType = "terminal"
		}
		dst[key] = value
	}
	return dst
}

func cloneSeedSettings(src map[string]SeedSetting) map[string]SeedSetting {
	dst := make(map[string]SeedSetting, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func normalizeEnrollmentState(state *SeedEnrollmentState) {
	if state.Offerings == nil {
		state.Offerings = make(map[string]int64)
	}
	if state.Settings == nil {
		state.Settings = make(map[string]json.RawMessage)
	}
}

func CloneEnrollmentState(src SeedEnrollmentState) SeedEnrollmentState {
	dst := SeedEnrollmentState{
		SchemaID: src.SchemaID, PhaseID: src.PhaseID, Offerings: make(map[string]int64, len(src.Offerings)),
		Requests:      append([]SeedEnrollmentRequest(nil), src.Requests...),
		ParentActions: append([]SeedParentPortalAction(nil), src.ParentActions...),
		Settings:      make(map[string]json.RawMessage, len(src.Settings)),
	}
	for key, value := range src.Offerings {
		dst.Offerings[key] = value
	}
	for key, value := range src.Settings {
		dst.Settings[key] = value
	}
	return dst
}

func SemanticKey(value string) string {
	replacer := strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue")
	value = strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
	var out strings.Builder
	dash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			dash = false
		} else if out.Len() > 0 && !dash {
			out.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(out.String(), "-")
}

type legacySeedStateV2 struct {
	Version     string                `json:"version"`
	CreatedAt   time.Time             `json:"created_at"`
	BaseURL     string                `json:"base_url"`
	DevicePIN   string                `json:"device_pin"`
	Bootstrap   SeedStateBootstrap    `json:"bootstrap"`
	Accounts    SeedStateAccounts     `json:"accounts"`
	Parents     []ParentCredentials   `json:"parents,omitempty"`
	Devices     map[string]SeedDevice `json:"devices"`
	Students    []SeedStudent         `json:"students"`
	Rooms       map[string]int64      `json:"rooms"`
	Activities  map[string]int64      `json:"activities"`
	Groups      map[string]int64      `json:"groups"`
	Enrollment  SeedEnrollmentState   `json:"enrollment,omitempty"`
	Credentials SeedStateCredentials  `json:"credentials"`
	Topology    SeedStateTopology     `json:"topology"`
	Entities    SeedStateEntities     `json:"entities"`
	Lookups     SeedStateLookups      `json:"lookups"`
	Scenarios   SeedStateScenarios    `json:"scenarios"`
}

func loadLegacySeedState(data []byte) (*SeedState, error) {
	var legacy legacySeedStateV2
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("unmarshal seed state: %w", err)
	}
	state := seedStateFromLegacy(legacy)
	applyLegacyNestedViews(state, legacy)
	state.Normalize()
	if _, ok := state.Profiles[state.DefaultProfile]; !ok {
		state.syncProfile(state.DefaultProfile)
	}
	return state, nil
}

func seedStateFromLegacy(legacy legacySeedStateV2) *SeedState {
	return &SeedState{
		Version: CurrentSeedStateVersion, CreatedAt: legacy.CreatedAt, BaseURL: legacy.BaseURL,
		DefaultProfile: DefaultProfileKey, DevicePIN: legacy.DevicePIN, Bootstrap: legacy.Bootstrap,
		Accounts: legacy.Accounts, Parents: legacy.Parents, Devices: legacy.Devices, Students: legacy.Students,
		Rooms: legacy.Rooms, Activities: legacy.Activities, Groups: legacy.Groups,
		Enrollment: legacy.Enrollment, Credentials: legacy.Credentials, Topology: legacy.Topology, Entities: legacy.Entities,
		Lookups: legacy.Lookups, Scenarios: legacy.Scenarios,
	}
}

func applyLegacyNestedViews(state *SeedState, legacy legacySeedStateV2) {
	if len(state.Devices) == 0 {
		state.Devices = legacy.Entities.Devices
	}
	if len(state.Students) == 0 {
		state.Students = legacy.Entities.Students
	}
	if len(state.Rooms) == 0 {
		state.Rooms = legacy.Lookups.Rooms
	}
	if len(state.Activities) == 0 {
		state.Activities = legacy.Lookups.Activities
	}
	if len(state.Groups) == 0 {
		state.Groups = legacy.Lookups.Groups
	}
	if state.Bootstrap.OrganizationID == 0 {
		state.Bootstrap = legacy.Entities.Bootstrap
	}
	if state.Enrollment.PhaseID == 0 {
		state.Enrollment = legacy.Entities.Enrollment
	}
}
