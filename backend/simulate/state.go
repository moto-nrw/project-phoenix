package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DefaultSeedStatePath = ".seed-state.json"

type SeedState struct {
	Version     string                `json:"version"`
	CreatedAt   time.Time             `json:"created_at"`
	BaseURL     string                `json:"base_url"`
	Bootstrap   SeedStateBootstrap    `json:"bootstrap"`
	DevicePIN   string                `json:"device_pin"`
	Accounts    SeedStateAccounts     `json:"accounts"`
	Devices     map[string]SeedDevice `json:"devices"`
	Students    []SeedStudent         `json:"students"`
	Rooms       map[string]int64      `json:"rooms"`
	Activities  map[string]int64      `json:"activities"`
	Groups      map[string]int64      `json:"groups"`
	Credentials struct {
		DevicePIN string            `json:"device_pin"`
		Accounts  SeedStateAccounts `json:"accounts"`
	} `json:"credentials"`
	Entities struct {
		Devices  map[string]SeedDevice `json:"devices"`
		Students []SeedStudent         `json:"students"`
	} `json:"entities"`
	Lookups struct {
		Rooms      map[string]int64 `json:"rooms"`
		Activities map[string]int64 `json:"activities"`
		Groups     map[string]int64 `json:"groups"`
	} `json:"lookups"`
}

type SeedStateBootstrap struct {
	TenantSlug string `json:"tenant_slug"`
}

type SeedStateAccounts struct {
	Admin    []AccountCredentials `json:"admin"`
	Betreuer []AccountCredentials `json:"betreuer"`
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

func LoadSeedState(path string) (*SeedState, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read seed state: %w", err)
	}
	var state SeedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal seed state: %w", err)
	}
	if state.Version != "" && state.Version != "2" {
		return nil, fmt.Errorf("unsupported seed state version: %s", state.Version)
	}
	state.normalize()
	return &state, nil
}

func (s *SeedState) normalize() {
	if s.Version == "" {
		s.Version = "2"
	}
	if s.DevicePIN == "" {
		s.DevicePIN = s.Credentials.DevicePIN
	}
	if len(s.Accounts.Admin) == 0 && len(s.Accounts.Betreuer) == 0 {
		s.Accounts = s.Credentials.Accounts
	}
	if len(s.Devices) == 0 {
		s.Devices = s.Entities.Devices
	}
	if len(s.Students) == 0 {
		s.Students = s.Entities.Students
	}
	if len(s.Rooms) == 0 {
		s.Rooms = s.Lookups.Rooms
	}
	if len(s.Activities) == 0 {
		s.Activities = s.Lookups.Activities
	}
	if len(s.Groups) == 0 {
		s.Groups = s.Lookups.Groups
	}
}
