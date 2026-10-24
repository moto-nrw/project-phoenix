package peopledirectory

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

func (sg *GuardianRelationshipSnapshot) Validate() error {
	value := contact.GuardianRelationshipValues{StudentID: sg.StudentID, GuardianProfileID: sg.GuardianProfileID, RelationshipType: sg.RelationshipType, GuardianRole: sg.GuardianRole, PermissionsPresent: sg.Permissions != nil}
	err := contact.NormalizeGuardianRelationship(&value)
	sg.RelationshipType, sg.GuardianRole = value.RelationshipType, value.GuardianRole
	if value.PermissionsPresent && sg.Permissions == nil {
		sg.Permissions = []byte(`{}`)
	}
	return err
}
