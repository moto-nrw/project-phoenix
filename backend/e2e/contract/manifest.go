package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ManifestPath        = ".e2e-manifest.json"
	ManifestVersion     = "4"
	ScenarioName        = "e2e-multi-tenant"
	DefaultBackendURL   = "http://localhost:8081"
	DefaultTenantDomain = "localtest.me"
	DefaultFrontendPort = 3030
)

// Manifest is the machine-readable contract between the Go-owned E2E world
// builder and the Playwright harness.
type Manifest struct {
	Version   string           `json:"version"`
	Runtime   Runtime          `json:"runtime"`
	Scenario  ScenarioMetadata `json:"scenario"`
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
}

type ScenarioMetadata struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
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
	SearchPair StudentPair `json:"search_pair"`
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
	m.Runtime.Normalize()
	if m.Scenario.Mode == "" {
		m.Scenario.Mode = "single-tenant"
	}
}

func (r *Runtime) Normalize() {
	if r == nil {
		return
	}
	if r.BackendURL == "" {
		r.BackendURL = DefaultBackendURL
	}
	if r.TenantDomain == "" {
		r.TenantDomain = DefaultTenantDomain
	}
	if r.FrontendPort == 0 {
		r.FrontendPort = DefaultFrontendPort
	}
	if r.OperatorHostname == "" {
		r.OperatorHostname = fmt.Sprintf("operator.%s:%d", r.TenantDomain, r.FrontendPort)
	}
}

func DefaultRuntime() Runtime {
	r := Runtime{}
	r.Normalize()
	return r
}

func WriteManifest(manifest *Manifest, path string) error {
	manifest.Normalize()
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
	return &manifest, nil
}
