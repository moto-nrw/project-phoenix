package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DefaultSeedStatePath = ".seed-state.json"

type SeedState struct {
	CreatedAt  time.Time             `json:"created_at"`
	BaseURL    string                `json:"base_url"`
	DevicePIN  string                `json:"device_pin"`
	Bootstrap  SeedStateBootstrap    `json:"bootstrap"`
	Accounts   SeedStateAccounts     `json:"accounts"`
	Devices    map[string]SeedDevice `json:"devices"`
	Students   []SeedStudent         `json:"students"`
	Rooms      map[string]int64      `json:"rooms"`
	Activities map[string]int64      `json:"activities"`
	Groups     map[string]int64      `json:"groups"`
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

func WriteSeedState(state *SeedState, path string) error {
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
	return &state, nil
}
