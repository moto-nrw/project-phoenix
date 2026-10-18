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

func (sg *GuardianRelationshipSnapshot) SetStudent(student *StudentState) {
	sg.Student = student
	if student != nil {
		sg.StudentID = student.ID
	}
}

func (sg *GuardianRelationshipSnapshot) GetRelationshipName() string {
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
