package users

import (
	"strings"
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
// relationship types. StudentGuardian.Validate and the student-create guardian
// path (services/users) resolve the allowed set through here so the request
// paths cannot drift apart.
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
