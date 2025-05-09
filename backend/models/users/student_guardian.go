package users

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// StudentGuardian represents the relationship between a student and their guardian
type StudentGuardian struct {
	base.Model
	StudentID          int64  `bun:"student_id,notnull" json:"student_id"`
	GuardianAccountID  int64  `bun:"guardian_account_id,notnull" json:"guardian_account_id"`
	RelationshipType   string `bun:"relationship_type,notnull" json:"relationship_type"`
	IsPrimary          bool   `bun:"is_primary,notnull" json:"is_primary"`
	IsEmergencyContact bool   `bun:"is_emergency_contact,notnull" json:"is_emergency_contact"`
	CanPickup          bool   `bun:"can_pickup,notnull" json:"can_pickup"`
	Permissions        string `bun:"permissions,notnull" json:"permissions,omitempty"` // JSON string

	// Relations not stored in the database
	Student *Student `bun:"-" json:"student,omitempty"`
	// GuardianAccount would be a reference to auth.AccountParent

	// Parsed permissions
	parsedPermissions map[string]interface{} `bun:"-" json:"-"`
}

// TableName returns the database table name
func (sg *StudentGuardian) TableName() string {
	return "users.students_guardians"
}

// Validate ensures student guardian data is valid
func (sg *StudentGuardian) Validate() error {
	if sg.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if sg.GuardianAccountID <= 0 {
		return errors.New("guardian account ID is required")
	}

	// Validate relationship type
	if sg.RelationshipType == "" {
		return errors.New("relationship type is required")
	}

	// Convert relationship type to lowercase for consistency
	sg.RelationshipType = strings.ToLower(sg.RelationshipType)

	// Validate against known types
	validTypes := map[string]bool{
		"parent":   true,
		"guardian": true,
		"relative": true,
		"other":    true,
	}

	if !validTypes[sg.RelationshipType] {
		return errors.New("invalid relationship type")
	}

	// Validate permissions JSON if provided
	if sg.Permissions != "" {
		var permissions map[string]interface{}
		if err := json.Unmarshal([]byte(sg.Permissions), &permissions); err != nil {
			return errors.New("invalid permissions JSON format")
		}
		sg.parsedPermissions = permissions
	} else {
		// Initialize with empty object
		sg.Permissions = "{}"
	}

	return nil
}

// SetStudent links this relationship to a student
func (sg *StudentGuardian) SetStudent(student *Student) {
	sg.Student = student
	if student != nil {
		sg.StudentID = student.ID
	}
}

// GetPermissions returns parsed permissions
func (sg *StudentGuardian) GetPermissions() map[string]interface{} {
	// Parse permissions if needed
	if sg.parsedPermissions == nil && sg.Permissions != "" {
		var permissions map[string]interface{}
		if err := json.Unmarshal([]byte(sg.Permissions), &permissions); err != nil {
			sg.parsedPermissions = make(map[string]interface{})
		} else {
			sg.parsedPermissions = permissions
		}
	}

	if sg.parsedPermissions == nil {
		sg.parsedPermissions = make(map[string]interface{})
	}

	return sg.parsedPermissions
}

// UpdatePermissions updates the permissions JSON
func (sg *StudentGuardian) UpdatePermissions(permissions map[string]interface{}) error {
	sg.parsedPermissions = permissions

	// Update the JSON string
	permissionsBytes, err := json.Marshal(sg.parsedPermissions)
	if err != nil {
		return err
	}

	sg.Permissions = string(permissionsBytes)
	return nil
}

// HasPermission checks if a specific permission exists and is true
func (sg *StudentGuardian) HasPermission(permission string) bool {
	permissions := sg.GetPermissions()

	value, exists := permissions[permission]
	if !exists {
		return false
	}

	// Try to convert to bool
	boolValue, ok := value.(bool)
	if ok {
		return boolValue
	}

	// If not a bool, check if it's a non-empty value
	return value != nil
}

// GetRelationshipName returns a formatted name for the relationship type
func (sg *StudentGuardian) GetRelationshipName() string {
	switch sg.RelationshipType {
	case "parent":
		return "Parent"
	case "guardian":
		return "Guardian"
	case "relative":
		return "Relative"
	case "other":
		return "Other"
	default:
		return "Unknown"
	}
}

// GetID implements the base.Entity interface
func (sg *StudentGuardian) GetID() interface{} {
	return sg.ID
}

// GetCreatedAt implements the base.Entity interface
func (sg *StudentGuardian) GetCreatedAt() time.Time {
	return sg.CreatedAt
}

// GetUpdatedAt implements the base.Entity interface
func (sg *StudentGuardian) GetUpdatedAt() time.Time {
	return sg.UpdatedAt
}
