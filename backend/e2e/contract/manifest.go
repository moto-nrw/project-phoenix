package contract

import (
	"encoding/json"
	"fmt"
	"net/url"
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
	if s.Version != StateVersion {
		return fmt.Errorf("state.version must be %s", StateVersion)
	}
	if s.Runtime.BackendURL == "" {
		return fmt.Errorf("state.runtime.backend_url is required")
	}
	if err := validateHTTPURL("state.runtime.backend_url", s.Runtime.BackendURL); err != nil {
		return err
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
	if err := validateTenant("state.world.tenants.primary", s.World.Tenants.Primary); err != nil {
		return err
	}
	if s.Setup.Auth.RequiresSecondaryTenant && s.World.Tenants.Secondary == nil {
		return fmt.Errorf("state.setup.auth.requires_secondary_tenant requires world.tenants.secondary")
	}
	if s.World.Tenants.Secondary != nil {
		if err := validateTenant("state.world.tenants.secondary", *s.World.Tenants.Secondary); err != nil {
			return err
		}
	}
	if err := validateActor("state.world.actors.admin", s.World.Actors.Admin); err != nil {
		return err
	}
	if err := validateActor("state.world.actors.staff", s.World.Actors.Staff); err != nil {
		return err
	}
	if s.World.Actors.Operator != nil {
		if s.World.Actors.Operator.Email == "" {
			return fmt.Errorf("state.world.actors.operator.email is required")
		}
		if s.World.Actors.Operator.Password == "" {
			return fmt.Errorf("state.world.actors.operator.password is required")
		}
	}
	if err := validateDevice("state.world.devices.default_checkin", s.World.Devices.DefaultCheckin); err != nil {
		return err
	}
	if err := validateStudentRef("state.fixtures.students.search_pair.primary", s.Fixtures.Students.SearchPair.Primary); err != nil {
		return err
	}
	if err := validateStudentRef("state.fixtures.students.search_pair.secondary", s.Fixtures.Students.SearchPair.Secondary); err != nil {
		return err
	}
	if err := validateStudentRef("state.fixtures.students.sick_present", s.Fixtures.Students.SickPresent); err != nil {
		return err
	}
	if err := validateGroupRef("state.fixtures.groups.visible_pair.primary", s.Fixtures.Groups.VisiblePair.Primary); err != nil {
		return err
	}
	if err := validateGroupRef("state.fixtures.groups.visible_pair.secondary", s.Fixtures.Groups.VisiblePair.Secondary); err != nil {
		return err
	}
	if err := validateCheckinFixture("state.fixtures.checkin", s.Fixtures.Checkin); err != nil {
		return err
	}
	if s.Setup.Auth.RequiresVerifiedSwitching && !s.Assertions.Switching.Verified {
		return fmt.Errorf("state.setup.auth.requires_verified_switching requires assertions.switching.verified")
	}
	if s.Assertions.Switching.Verified {
		if s.Assertions.Switching.LinkEmail == "" {
			return fmt.Errorf("state.assertions.switching.link_email is required when switching is verified")
		}
		if s.Assertions.Switching.Actor == nil {
			return fmt.Errorf("state.assertions.switching.actor is required when switching is verified")
		}
		if s.Assertions.Switching.Actor != nil {
			if err := validateActor("state.assertions.switching.actor", *s.Assertions.Switching.Actor); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateHTTPURL(field, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", field, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", field)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", field)
	}
	return nil
}

func validateTenant(field string, tenant Tenant) error {
	if tenant.OrganizationID <= 0 {
		return fmt.Errorf("%s.organization_id must be > 0", field)
	}
	if tenant.SchoolID <= 0 {
		return fmt.Errorf("%s.school_id must be > 0", field)
	}
	if tenant.Slug == "" {
		return fmt.Errorf("%s.slug is required", field)
	}
	if tenant.Name == "" {
		return fmt.Errorf("%s.name is required", field)
	}
	return nil
}

func validateActor(field string, actor Actor) error {
	if actor.Email == "" {
		return fmt.Errorf("%s.email is required", field)
	}
	if actor.Password == "" {
		return fmt.Errorf("%s.password is required", field)
	}
	if actor.DisplayName == "" {
		return fmt.Errorf("%s.display_name is required", field)
	}
	if actor.Role == "" {
		return fmt.Errorf("%s.role is required", field)
	}
	if actor.StaffID <= 0 {
		return fmt.Errorf("%s.staff_id must be > 0", field)
	}
	return nil
}

func validateDevice(field string, device Device) error {
	if device.Key == "" {
		return fmt.Errorf("%s.key is required", field)
	}
	if device.APIKey == "" {
		return fmt.Errorf("%s.api_key is required", field)
	}
	if device.PIN == "" {
		return fmt.Errorf("%s.pin is required", field)
	}
	return nil
}

func validateStudentRef(field string, student StudentRef) error {
	if student.ID <= 0 {
		return fmt.Errorf("%s.id must be > 0", field)
	}
	if student.FirstName == "" {
		return fmt.Errorf("%s.first_name is required", field)
	}
	if student.LastName == "" {
		return fmt.Errorf("%s.last_name is required", field)
	}
	if student.GroupKey == "" {
		return fmt.Errorf("%s.group_key is required", field)
	}
	if student.Class == "" {
		return fmt.Errorf("%s.class is required", field)
	}
	return nil
}

func validateGroupRef(field string, group GroupRef) error {
	if group.ID <= 0 {
		return fmt.Errorf("%s.id must be > 0", field)
	}
	if group.Key == "" {
		return fmt.Errorf("%s.key is required", field)
	}
	if group.DisplayName == "" {
		return fmt.Errorf("%s.display_name is required", field)
	}
	return nil
}

func validateCheckinFixture(field string, fixture CheckinFixture) error {
	if err := validateStudentRef(field+".student", fixture.Student); err != nil {
		return err
	}
	if fixture.Room.ID <= 0 {
		return fmt.Errorf("%s.room.id must be > 0", field)
	}
	if fixture.Room.Name == "" {
		return fmt.Errorf("%s.room.name is required", field)
	}
	if fixture.Activity.ID <= 0 {
		return fmt.Errorf("%s.activity.id must be > 0", field)
	}
	if fixture.Activity.Name == "" {
		return fmt.Errorf("%s.activity.name is required", field)
	}
	if fixture.DeviceKey == "" {
		return fmt.Errorf("%s.device_key is required", field)
	}
	if fixture.RFIDTag == "" {
		return fmt.Errorf("%s.rfid_tag is required", field)
	}
	if err := validateActor(field+".supervisor", fixture.Supervisor); err != nil {
		return err
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
