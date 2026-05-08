package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ManifestPath    = ".e2e-manifest.json"
	ManifestVersion = "7"
)

// Manifest is the machine-readable contract between the Go-owned E2E world
// builder and the Playwright harness.
type Manifest struct {
	Version   string           `json:"version"`
	Runtime   Runtime          `json:"runtime"`
	Scenario  ScenarioMetadata `json:"scenario"`
	Setup     Setup            `json:"setup"`
	Tenants   Tenants          `json:"tenants"`
	Actors    Actors           `json:"actors"`
	Switching *Switching       `json:"switching,omitempty"`
	Devices   Devices          `json:"devices"`
	Fixtures  Fixtures         `json:"fixtures"`
}

type Runtime struct {
	BackendURL       string `json:"backend_url"`
	TenantDomain     string `json:"tenant_domain"`
	FrontendPort     int    `json:"frontend_port"`
	OperatorHostname string `json:"operator_hostname"`
	NextAuthSecret   string `json:"nextauth_secret"`
	AuthTrustHost    bool   `json:"auth_trust_host"`
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

type Switching struct {
	LinkEmail string `json:"link_email"`
	Verified  bool   `json:"verified"`
	Actor     Actor  `json:"actor"`
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
	SearchPair   StudentPair `json:"search_pair"`
	PresentReady StudentRef  `json:"present_ready"`
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

type OperatorCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (m *Manifest) Normalize() {
	if m == nil {
		return
	}
	if m.Version == "" {
		m.Version = ManifestVersion
	}
}

func (m *Manifest) Validate() error {
	if m == nil {
		return fmt.Errorf("manifest is nil")
	}
	if m.Runtime.BackendURL == "" {
		return fmt.Errorf("manifest.runtime.backend_url is required")
	}
	if m.Runtime.TenantDomain == "" {
		return fmt.Errorf("manifest.runtime.tenant_domain is required")
	}
	if m.Runtime.FrontendPort <= 0 {
		return fmt.Errorf("manifest.runtime.frontend_port must be > 0")
	}
	if m.Runtime.OperatorHostname == "" {
		return fmt.Errorf("manifest.runtime.operator_hostname is required")
	}
	if m.Runtime.NextAuthSecret == "" {
		return fmt.Errorf("manifest.runtime.nextauth_secret is required")
	}
	if m.Scenario.Name == "" {
		return fmt.Errorf("manifest.scenario.name is required")
	}
	if m.Scenario.Mode == "" {
		return fmt.Errorf("manifest.scenario.mode is required")
	}
	if len(m.Setup.Auth.Roles) == 0 {
		return fmt.Errorf("manifest.setup.auth.roles must not be empty")
	}
	if m.Tenants.Primary.Slug == "" {
		return fmt.Errorf("manifest.tenants.primary.slug is required")
	}
	if m.Actors.Admin.Email == "" {
		return fmt.Errorf("manifest.actors.admin.email is required")
	}
	if m.Actors.Staff.Email == "" {
		return fmt.Errorf("manifest.actors.staff.email is required")
	}
	if m.Devices.DefaultCheckin.Key == "" {
		return fmt.Errorf("manifest.devices.default_checkin.key is required")
	}
	if m.Devices.DefaultCheckin.APIKey == "" {
		return fmt.Errorf("manifest.devices.default_checkin.api_key is required")
	}
	if m.Devices.DefaultCheckin.PIN == "" {
		return fmt.Errorf("manifest.devices.default_checkin.pin is required")
	}
	if m.Fixtures.Checkin.RFIDTag == "" {
		return fmt.Errorf("manifest.fixtures.checkin.rfid_tag is required")
	}
	return nil
}

func WriteManifest(manifest *Manifest, path string) error {
	manifest.Normalize()
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("validate e2e manifest: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal e2e manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write e2e manifest: %w", err)
	}
	return nil
}

func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read e2e manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("unmarshal e2e manifest: %w", err)
	}
	if manifest.Version != "" && manifest.Version != ManifestVersion {
		return nil, fmt.Errorf("unsupported e2e manifest version: %s", manifest.Version)
	}
	manifest.Normalize()
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validate e2e manifest: %w", err)
	}
	return &manifest, nil
}
