package users

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// RelationshipType represents the type of relationship between a person and guardian
type RelationshipType string

const (
	RelationshipParent   RelationshipType = "parent"
	RelationshipGuardian RelationshipType = "guardian"
	RelationshipRelative RelationshipType = "relative"
	RelationshipOther    RelationshipType = "other"
)

// IsValidRelationshipType delegates to the canonical contact policy.
func IsValidRelationshipType(s string) bool { return contact.IsValidRelationshipType(s) }
