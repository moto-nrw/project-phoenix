package users

import (
	"encoding/json"
	"errors"
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

// PersonGuardian represents the relationship between a person and their guardian
type PersonGuardian struct {
	base.Model
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

// TableName returns the database table name
func (pg *PersonGuardian) TableName() string {
	return "users.persons_guardians"
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

	// Validate against known types
	validTypes := map[RelationshipType]bool{
		RelationshipParent:   true,
		RelationshipGuardian: true,
		RelationshipRelative: true,
		RelationshipOther:    true,
	}

	if !validTypes[pg.RelationshipType] {
		return errors.New("invalid relationship type")
	}

	// Validate permissions JSON if provided
	if pg.Permissions != "" {
		var permissions map[string]bool
		if err := json.Unmarshal([]byte(pg.Permissions), &permissions); err != nil {
			return errors.New("invalid permissions JSON format")
		}
		pg.parsedPermissions = permissions
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

// HasPermission checks if the guardian has a specific permission
func (pg *PersonGuardian) HasPermission(permission string) bool {
	// Parse permissions if needed
	if pg.parsedPermissions == nil && pg.Permissions != "" {
		_ = json.Unmarshal([]byte(pg.Permissions), &pg.parsedPermissions)
	}

	if pg.parsedPermissions == nil {
		return false
	}

	return pg.parsedPermissions[permission]
}

// GrantPermission grants a specific permission to the guardian
func (pg *PersonGuardian) GrantPermission(permission string) error {
	// Parse permissions if needed
	if pg.parsedPermissions == nil {
		if pg.Permissions != "" {
			if err := json.Unmarshal([]byte(pg.Permissions), &pg.parsedPermissions); err != nil {
				pg.parsedPermissions = make(map[string]bool)
			}
		} else {
			pg.parsedPermissions = make(map[string]bool)
		}
	}

	// Set the permission
	pg.parsedPermissions[permission] = true

	// Update the JSON string
	permissionsBytes, err := json.Marshal(pg.parsedPermissions)
	if err != nil {
		return err
	}

	pg.Permissions = string(permissionsBytes)
	return nil
}

// RevokePermission revokes a specific permission from the guardian
func (pg *PersonGuardian) RevokePermission(permission string) error {
	// Parse permissions if needed
	if pg.parsedPermissions == nil && pg.Permissions != "" {
		_ = json.Unmarshal([]byte(pg.Permissions), &pg.parsedPermissions)
	}

	if pg.parsedPermissions == nil {
		return nil
	}

	// Remove the permission
	delete(pg.parsedPermissions, permission)

	// Update the JSON string
	permissionsBytes, err := json.Marshal(pg.parsedPermissions)
	if err != nil {
		return err
	}

	pg.Permissions = string(permissionsBytes)
	return nil
}

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
