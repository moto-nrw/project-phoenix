package simulate

import (
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/demoprofile"
)

const DefaultSeedStatePath = demoprofile.DefaultSeedStatePath
const DefaultProfileKey = demoprofile.DefaultProfileKey

type SeedState struct {
	Version     string                `json:"version"`
	CreatedAt   time.Time             `json:"created_at"`
	BaseURL     string                `json:"base_url"`
	ProfileKey  string                `json:"profile_key,omitempty"`
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

func LoadSeedStateProfile(path, profileKey string) (*SeedState, error) {
	contract, err := demoprofile.LoadSeedState(path)
	if err != nil {
		return nil, err
	}
	profile, err := contract.SelectProfile(profileKey)
	if err != nil {
		return nil, err
	}
	state := seedStateFromProfile(contract, profile)
	state.normalize()
	return state, nil
}

func seedStateFromProfile(contract *demoprofile.SeedState, profile *demoprofile.SeedProfile) *SeedState {
	state := &SeedState{
		Version: contract.Version, CreatedAt: contract.CreatedAt, BaseURL: contract.BaseURL,
		ProfileKey: profile.Key, Bootstrap: SeedStateBootstrap{TenantSlug: profile.School.TenantSlug},
		DevicePIN: profile.Credentials.DevicePIN,
		Accounts: SeedStateAccounts{
			Admin:    simulationAccounts(profile.Credentials.Accounts.Admin),
			Betreuer: simulationAccounts(profile.Credentials.Accounts.Betreuer),
		},
		Devices:    make(map[string]SeedDevice),
		Rooms:      simulationEntityIDs(profile.Entities.Rooms),
		Activities: simulationEntityIDs(profile.Entities.Activities),
		Groups:     simulationEntityIDs(profile.Entities.Groups),
	}
	for key, device := range profile.Devices {
		if device.APIKey != "" {
			state.Devices[key] = SeedDevice{APIKey: device.APIKey, Name: device.Name}
		}
	}
	studentKeys := make([]string, 0, len(profile.Entities.Students))
	for key := range profile.Entities.Students {
		studentKeys = append(studentKeys, key)
	}
	sort.Strings(studentKeys)
	for _, key := range studentKeys {
		student := profile.Entities.Students[key]
		state.Students = append(state.Students, SeedStudent{
			ID: student.ID, FirstName: student.FirstName, LastName: student.LastName,
			GroupKey: student.GroupKey, Class: student.Class,
		})
	}
	return state
}

func simulationAccounts(accounts []demoprofile.AccountCredentials) []AccountCredentials {
	result := make([]AccountCredentials, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, AccountCredentials{
			Email: account.Email, Password: account.Password, PIN: account.PIN, Name: account.Name,
			StaffID: account.StaffID, TeacherID: account.TeacherID, Group: account.Group,
		})
	}
	return result
}

func simulationEntityIDs(entities map[string]demoprofile.SeedEntityRef) map[string]int64 {
	result := make(map[string]int64, len(entities))
	for key, entity := range entities {
		name := entity.Name
		if name == "" {
			name = key
		}
		result[name] = entity.ID
	}
	return result
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
