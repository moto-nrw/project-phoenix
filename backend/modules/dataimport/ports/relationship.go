package ports

import "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

// MapRelationshipType converts the German relationship label of an imported
// guardian to its stored type through People Directory's relationship
// vocabulary.
func MapRelationshipType(label string) string {
	return peopledirectory.MapGuardianRelationshipType(label)
}
