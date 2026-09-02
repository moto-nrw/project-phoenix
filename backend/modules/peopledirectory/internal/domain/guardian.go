package domain

import (
	"strconv"
	"time"
)

// Guardian is the users.guardian_profiles row other owners may see through
// the directory: identity, contact preferences and the account link. Phone
// numbers stay with the application-level provider.
type Guardian struct {
	ID                     int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
	TenantID               int64
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
	Notes                  *string
	HasAccount             bool
	AccountID              *int64
}

// GuardianLink is one users.students_guardians row. Permissions carries the
// granted parents-portal permission names.
type GuardianLink struct {
	ID                 int64
	TenantID           int64
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        []string
}

// GrantedPermissions reduces the stored permissions object to the names
// whose value reads as true. The legacy SQL used
// COALESCE((permissions ->> name)::boolean, false), so booleans, boolean
// strings and non-zero numbers count as granted.
func GrantedPermissions(raw map[string]any) []string {
	names := make([]string, 0, len(raw))
	for name, value := range raw {
		if permissionGranted(value) {
			names = append(names, name)
		}
	}
	return names
}

func permissionGranted(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		granted, err := strconv.ParseBool(typed)
		return err == nil && granted
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	default:
		return false
	}
}
