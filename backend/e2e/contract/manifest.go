package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	StatePath    = ".e2e-state.json"
	StateVersion = "1"
)

// State is the single machine-readable contract between the Go-owned E2E
// world builder, the Playwright setup project, and the Go orchestrator.
type State struct {
	Version    string     `json:"version"`
	Runtime    Runtime    `json:"runtime"`
	World      World      `json:"world"`
	Setup      Setup      `json:"setup"`
	Fixtures   Fixtures   `json:"fixtures"`
	Assertions Assertions `json:"assertions"`
}

type Runtime struct {
	BackendURL       string `json:"backend_url"`
	TenantDomain     string `json:"tenant_domain"`
	FrontendPort     int    `json:"frontend_port"`
	OperatorHostname string `json:"operator_hostname"`
	NextAuthSecret   string `json:"nextauth_secret"`
	AuthTrustHost    bool   `json:"auth_trust_host"`
}

type World struct {
	Scenario ScenarioMetadata `json:"scenario"`
	Tenants  Tenants          `json:"tenants"`
	Actors   Actors           `json:"actors"`
	Devices  Devices          `json:"devices"`
}

type ScenarioMetadata struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type Setup struct {
	Auth AuthSetup `json:"auth"`
}

type AuthSetup struct {
	Roles                     []string `json:"roles"`
	RequiresSecondaryTenant   bool     `json:"requires_secondary_tenant"`
	RequiresVerifiedSwitching bool     `json:"requires_verified_switching"`
}

type Tenants struct {
	Primary   Tenant  `json:"primary"`
	Secondary *Tenant `json:"secondary,omitempty"`
}

type Tenant struct {
	OrganizationID int64  `json:"organization_id"`
	SchoolID       int64  `json:"school_id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
}

type Actors struct {
	Admin    Actor                `json:"admin"`
	Staff    Actor                `json:"staff"`
	Operator *OperatorCredentials `json:"operator,omitempty"`
}

type Actor struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	StaffID     int64  `json:"staff_id"`
}

type Devices struct {
	DefaultCheckin Device `json:"default_checkin"`
}

type Device struct {
	Key    string `json:"key"`
	APIKey string `json:"api_key"`
	PIN    string `json:"pin"`
}

type Fixtures struct {
	Students StudentFixtures `json:"students"`
	Groups   GroupFixtures   `json:"groups"`
	Checkin  CheckinFixture  `json:"checkin"`
}

type StudentFixtures struct {
	SearchPair  StudentPair `json:"search_pair"`
	SickPresent StudentRef  `json:"sick_present"`
}

type StudentPair struct {
	Primary   StudentRef `json:"primary"`
	Secondary StudentRef `json:"secondary"`
}

type StudentRef struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	GroupKey  string `json:"group_key"`
	Class     string `json:"class"`
}

type GroupFixtures struct {
	VisiblePair GroupPair `json:"visible_pair"`
}

type GroupPair struct {
	Primary   GroupRef `json:"primary"`
	Secondary GroupRef `json:"secondary"`
}

type GroupRef struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
}

type RoomRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ActivityRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CheckinFixture struct {
	Student    StudentRef  `json:"student"`
	Room       RoomRef     `json:"room"`
	Activity   ActivityRef `json:"activity"`
	DeviceKey  string      `json:"device_key"`
	RFIDTag    string      `json:"rfid_tag"`
	Supervisor Actor       `json:"supervisor"`
}

type Assertions struct {
	Switching SwitchingAssertion `json:"switching"`
}

type SwitchingAssertion struct {
	Required  bool   `json:"required"`
	Verified  bool   `json:"verified"`
	LinkEmail string `json:"link_email,omitempty"`
	Actor     *Actor `json:"actor,omitempty"`
}

type OperatorCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *State) Normalize() {
	if s == nil {
		return
	}
	if s.Version == "" {
		s.Version = StateVersion
	}
}

func (s *State) Validate() error {
	if s == nil {
		return fmt.Errorf("state is nil")
	}
	if s.Runtime.BackendURL == "" {
		return fmt.Errorf("state.runtime.backend_url is required")
	}
	if s.Runtime.TenantDomain == "" {
		return fmt.Errorf("state.runtime.tenant_domain is required")
	}
	if s.Runtime.FrontendPort <= 0 {
		return fmt.Errorf("state.runtime.frontend_port must be > 0")
	}
	if s.Runtime.OperatorHostname == "" {
		return fmt.Errorf("state.runtime.operator_hostname is required")
	}
	if s.Runtime.NextAuthSecret == "" {
		return fmt.Errorf("state.runtime.nextauth_secret is required")
	}
	if s.World.Scenario.Name == "" {
		return fmt.Errorf("state.world.scenario.name is required")
	}
	if s.World.Scenario.Mode == "" {
		return fmt.Errorf("state.world.scenario.mode is required")
	}
	if len(s.Setup.Auth.Roles) == 0 {
		return fmt.Errorf("state.setup.auth.roles must not be empty")
	}
	if s.World.Tenants.Primary.Slug == "" {
		return fmt.Errorf("state.world.tenants.primary.slug is required")
	}
	if s.World.Actors.Admin.Email == "" {
		return fmt.Errorf("state.world.actors.admin.email is required")
	}
	if s.World.Actors.Staff.Email == "" {
		return fmt.Errorf("state.world.actors.staff.email is required")
	}
	if s.World.Devices.DefaultCheckin.Key == "" {
		return fmt.Errorf("state.world.devices.default_checkin.key is required")
	}
	if s.World.Devices.DefaultCheckin.APIKey == "" {
		return fmt.Errorf("state.world.devices.default_checkin.api_key is required")
	}
	if s.World.Devices.DefaultCheckin.PIN == "" {
		return fmt.Errorf("state.world.devices.default_checkin.pin is required")
	}
	if s.Fixtures.Checkin.RFIDTag == "" {
		return fmt.Errorf("state.fixtures.checkin.rfid_tag is required")
	}
	if s.Setup.Auth.RequiresSecondaryTenant && s.World.Tenants.Secondary == nil {
		return fmt.Errorf("state.setup.auth.requires_secondary_tenant requires world.tenants.secondary")
	}
	if s.Setup.Auth.RequiresVerifiedSwitching && !s.Assertions.Switching.Verified {
		return fmt.Errorf("state.setup.auth.requires_verified_switching requires assertions.switching.verified")
	}
	return nil
}

func WriteState(state *State, path string) error {
	state.Normalize()
	if err := state.Validate(); err != nil {
		return fmt.Errorf("validate e2e state: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal e2e state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write e2e state: %w", err)
	}
	return nil
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read e2e state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal e2e state: %w", err)
	}
	if state.Version != "" && state.Version != StateVersion {
		return nil, fmt.Errorf("unsupported e2e state version: %s", state.Version)
	}
	state.Normalize()
	if err := state.Validate(); err != nil {
		return nil, fmt.Errorf("validate e2e state: %w", err)
	}
	return &state, nil
}
