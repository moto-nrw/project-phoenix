package users

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RelationshipType represents the type of relationship between a person and guardian
type RelationshipType string

const (
	RelationshipParent   RelationshipType = "parent"
	RelationshipGuardian RelationshipType = "guardian"
	RelationshipRelative RelationshipType = "relative"
	RelationshipOther    RelationshipType = "other"
)

// validRelationshipTypes is the single source of truth for the allowed guardian
// relationship types. PersonGuardian.Validate, StudentGuardian.Validate, and the
// student-create guardian path (services/users) all resolve the allowed set
// through here so the three request paths cannot drift apart.
var validRelationshipTypes = map[RelationshipType]bool{
	RelationshipParent:   true,
	RelationshipGuardian: true,
	RelationshipRelative: true,
	RelationshipOther:    true,
}

// IsValidRelationshipType reports whether s names an allowed guardian
// relationship type. Input is trimmed and lowercased before the lookup, matching
// the normalization the model Validate() methods apply.
func IsValidRelationshipType(s string) bool {
	return validRelationshipTypes[RelationshipType(strings.ToLower(strings.TrimSpace(s)))]
}

// PersonGuardian represents the relationship between a person and their guardian
type PersonGuardian struct {
	base.Model `bun:"schema:users,table:persons_guardians"`
	base.TenantModel
	PersonID          int64            `bun:"person_id,notnull" json:"person_id"`
	GuardianAccountID int64            `bun:"guardian_account_id,notnull" json:"guardian_account_id"`
	RelationshipType  RelationshipType `bun:"relationship_type,notnull" json:"relationship_type"`
	IsPrimary         bool             `bun:"is_primary,notnull" json:"is_primary"`
	Permissions       string           `bun:"permissions" json:"permissions,omitempty"` // JSON string

	// Relations not stored in the database
	Person *Person `bun:"-" json:"person,omitempty"`
	// GuardianAccount would be a reference to auth.AccountParent which isn't implemented yet

	// Parsed permissions
	parsedPermissions map[string]bool `bun:"-" json:"-"`
}

// Validate ensures person guardian data is valid
func (pg *PersonGuardian) Validate() error {
	if pg.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if pg.GuardianAccountID <= 0 {
		return errors.New("guardian account ID is required")
	}

	// Validate relationship type
	if pg.RelationshipType == "" {
		return errors.New("relationship type is required")
	}

	// Convert relationship type to lowercase for consistency
	pg.RelationshipType = RelationshipType(strings.ToLower(string(pg.RelationshipType)))

	// Validate against the shared allowed set (single source of truth)
	if !validRelationshipTypes[pg.RelationshipType] {
		return errors.New("invalid relationship type")
	}

	// Validate permissions JSON if provided
	if pg.Permissions != "" {
		if err := json.Unmarshal([]byte(pg.Permissions), &pg.parsedPermissions); err != nil {
			return fmt.Errorf("invalid permissions JSON format: %w", err)
		}
	}

	return nil
}

// SetPerson links this relationship to a person
func (pg *PersonGuardian) SetPerson(person *Person) {
	pg.Person = person
	if person != nil {
		pg.PersonID = person.ID
	}
}

// Note: the guardian permission check and grant/revoke mutations (formerly
// PersonGuardian.HasPermission / GrantPermission / RevokePermission) moved out
// of the model in issue #586 (Rule 12) to:
//   - auth/authorize/guardian_permission.go (PersonGuardianHasPermission /
//     PersonGuardianGrantPermission / PersonGuardianRevokePermission)

// GetRelationshipName returns a formatted name for the relationship type
func (pg *PersonGuardian) GetRelationshipName() string {
	switch pg.RelationshipType {
	case RelationshipParent:
		return "Parent"
	case RelationshipGuardian:
		return "Guardian"
	case RelationshipRelative:
		return "Relative"
	case RelationshipOther:
		return "Other"
	default:
		return "Unknown"
	}
}
