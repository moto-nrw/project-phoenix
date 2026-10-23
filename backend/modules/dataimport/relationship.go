package dataimport

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"

// MapRelationshipType converts German relationship types to valid English
// types through People Directory's relationship vocabulary.
func MapRelationshipType(germanType string) string {
	return contact.MapRelationshipType(germanType)
}
