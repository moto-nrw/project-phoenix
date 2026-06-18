package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DefaultSeedStatePath = ".seed-state.json"
const CurrentSeedStateVersion = "2"

type SeedState struct {
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

type SeedStateCredentials struct {
	Operator  *SeedOperatorCredentials `json:"operator,omitempty"`
	DevicePIN string                   `json:"device_pin"`
	Accounts  SeedStateAccounts        `json:"accounts"`
	Parents   []ParentCredentials      `json:"parents,omitempty"`
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
	Email     string `json:"email"`
	Password  string `json:"password"`
	PIN       string `json:"pin"`
	Name      string `json:"name"`
	StaffID   int64  `json:"staff_id"`
	TeacherID int64  `json:"teacher_id,omitempty"`
	Group     string `json:"group,omitempty"`
}

type SeedDevice struct {
	APIKey string `json:"api_key"`
	Name   string `json:"name"`
}

type SeedStudent struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	GroupKey  string `json:"group_key"`
	Class     string `json:"class"`
}

type SeedEnrollmentState struct {
	PhaseID       int64                    `json:"phase_id,omitempty"`
	Offerings     map[string]int64         `json:"offerings,omitempty"`
	Requests      []SeedEnrollmentRequest  `json:"requests,omitempty"`
	ParentActions []SeedParentPortalAction `json:"parent_actions,omitempty"`
	Settings      map[string]any           `json:"settings,omitempty"`
}

type SeedEnrollmentRequest struct {
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
	state.Normalize()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seed state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write seed state: %w", err)
	}
	return nil
}

func LoadSeedState(path string) (*SeedState, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read seed state: %w", err)
	}
	var state SeedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal seed state: %w", err)
	}
	if state.Version != "" && state.Version != CurrentSeedStateVersion {
		return nil, fmt.Errorf("unsupported seed state version: %s", state.Version)
	}
	state.Normalize()
	return &state, nil
}

func (s *SeedState) Normalize() {
	if s == nil {
		return
	}

	if s.Version == "" {
		s.Version = CurrentSeedStateVersion
	}
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
	if s.Entities.Devices == nil {
		s.Entities.Devices = make(map[string]SeedDevice)
	}
	if s.Lookups.Rooms == nil {
		s.Lookups.Rooms = make(map[string]int64)
	}
	if s.Lookups.Activities == nil {
		s.Lookups.Activities = make(map[string]int64)
	}
	if s.Lookups.Groups == nil {
		s.Lookups.Groups = make(map[string]int64)
	}

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
	if len(s.Credentials.Parents) == 0 && len(s.Parents) > 0 {
		s.Credentials.Parents = append([]ParentCredentials(nil), s.Parents...)
	}
	if len(s.Parents) == 0 && len(s.Credentials.Parents) > 0 {
		s.Parents = append([]ParentCredentials(nil), s.Credentials.Parents...)
	}

	if s.Entities.Bootstrap.OrganizationID == 0 && s.Bootstrap.OrganizationID != 0 {
		s.Entities.Bootstrap = s.Bootstrap
	}
	if s.Bootstrap.OrganizationID == 0 && s.Entities.Bootstrap.OrganizationID != 0 {
		s.Bootstrap = s.Entities.Bootstrap
	}

	if len(s.Entities.Devices) == 0 && len(s.Devices) > 0 {
		s.Entities.Devices = cloneSeedDevices(s.Devices)
	}
	if len(s.Devices) == 0 && len(s.Entities.Devices) > 0 {
		s.Devices = cloneSeedDevices(s.Entities.Devices)
	}

	if len(s.Entities.Students) == 0 && len(s.Students) > 0 {
		s.Entities.Students = append([]SeedStudent(nil), s.Students...)
	}
	if len(s.Students) == 0 && len(s.Entities.Students) > 0 {
		s.Students = append([]SeedStudent(nil), s.Entities.Students...)
	}
	normalizeEnrollmentState(&s.Enrollment)
	normalizeEnrollmentState(&s.Entities.Enrollment)
	if s.Entities.Enrollment.PhaseID == 0 && s.Enrollment.PhaseID != 0 {
		s.Entities.Enrollment = cloneEnrollmentState(s.Enrollment)
	}
	if s.Enrollment.PhaseID == 0 && s.Entities.Enrollment.PhaseID != 0 {
		s.Enrollment = cloneEnrollmentState(s.Entities.Enrollment)
	}

	if len(s.Lookups.Rooms) == 0 && len(s.Rooms) > 0 {
		s.Lookups.Rooms = cloneSeedIDMap(s.Rooms)
	}
	if len(s.Rooms) == 0 && len(s.Lookups.Rooms) > 0 {
		s.Rooms = cloneSeedIDMap(s.Lookups.Rooms)
	}

	if len(s.Lookups.Activities) == 0 && len(s.Activities) > 0 {
		s.Lookups.Activities = cloneSeedIDMap(s.Activities)
	}
	if len(s.Activities) == 0 && len(s.Lookups.Activities) > 0 {
		s.Activities = cloneSeedIDMap(s.Lookups.Activities)
	}

	if len(s.Lookups.Groups) == 0 && len(s.Groups) > 0 {
		s.Lookups.Groups = cloneSeedIDMap(s.Groups)
	}
	if len(s.Groups) == 0 && len(s.Lookups.Groups) > 0 {
		s.Groups = cloneSeedIDMap(s.Lookups.Groups)
	}

	if s.Topology.Organizations == 0 && s.Bootstrap.OrganizationID != 0 {
		s.Topology.Organizations = 1
	}
	if s.Topology.Schools == 0 && s.Bootstrap.SchoolID != 0 {
		s.Topology.Schools = 1
	}
	if s.Topology.Mode == "" {
		s.Topology.Mode = "full-demo"
	}
	if s.Scenarios.DefaultPlayer == "" {
		s.Scenarios.DefaultPlayer = "pyreportal"
	}
	if s.Scenarios.DefaultMode == "" {
		s.Scenarios.DefaultMode = "hybrid"
	}
}

func cloneSeedDevices(src map[string]SeedDevice) map[string]SeedDevice {
	dst := make(map[string]SeedDevice, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneSeedIDMap(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func normalizeEnrollmentState(state *SeedEnrollmentState) {
	if state == nil {
		return
	}
	if state.Offerings == nil {
		state.Offerings = make(map[string]int64)
	}
	if state.Settings == nil {
		state.Settings = make(map[string]any)
	}
}

func cloneEnrollmentState(src SeedEnrollmentState) SeedEnrollmentState {
	dst := SeedEnrollmentState{
		PhaseID:       src.PhaseID,
		Offerings:     cloneSeedIDMap(src.Offerings),
		Requests:      append([]SeedEnrollmentRequest(nil), src.Requests...),
		ParentActions: append([]SeedParentPortalAction(nil), src.ParentActions...),
		Settings:      make(map[string]any, len(src.Settings)),
	}
	for key, value := range src.Settings {
		dst.Settings[key] = value
	}
	return dst
}
