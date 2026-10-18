package users

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func NativeStudentRosterEntry(value *models.StudentWithGroupInfo) *directory.StudentRosterEntry {
	if value == nil {
		return nil
	}
	result := &directory.StudentRosterEntry{Record: NativeStudentState(value.Student), GroupName: value.GroupName}
	if value.Student != nil && value.Person != nil {
		result.FirstName = value.Person.FirstName
		result.LastName = value.Person.LastName
		result.TagID = value.Person.TagID
		result.AccountID = value.Person.AccountID
	}
	return result
}
