package services

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The adapter in this file serves the staff lookups of the public Workforce
// personnel-record contract from the retained person service (#2690). Lookups
// pass their failures through unchanged so the not-found classification of the
// HTTP layer keeps working. The personnel-record administration itself is
// Workforce's own (workforce.StaffAdmin, #3752) and is embedded unchanged.

type staffDirectoryCapability struct {
	workforce.StaffRecordAdmin
	people *peopleCompose.PersonDirectory
}

// StaffDirectoryCapability serves workforce.StaffDirectory: the lookups from
// the retained person service, the administration from Workforce.
func StaffDirectoryCapability(people *peopleCompose.PersonDirectory, admin workforce.StaffRecordAdmin) workforce.StaffDirectory {
	if people == nil || admin == nil {
		panic("staff directory capability: person service and personnel-record administration are required")
	}
	return staffDirectoryCapability{StaffRecordAdmin: admin, people: people}
}

func publicPerson(entity *userModels.Person) *workforce.Person {
	if entity == nil {
		return nil
	}
	return &workforce.Person{
		ID: entity.ID, FirstName: entity.FirstName, LastName: entity.LastName, Birthday: publicOptionalDate(entity.Birthday), AccountID: entity.AccountID,
	}
}

func publicStaffProfile(entity *userModels.Staff) *workforce.StaffProfile {
	if entity == nil {
		return nil
	}
	profile := &workforce.StaffProfile{
		ID: entity.ID, PersonID: entity.PersonID, EmploymentType: entity.EmploymentType, WorkTimeModelID: entity.WorkTimeModelID,
		RotationAnchorDate: publicOptionalDate(entity.RotationAnchorDate), PersonnelNumber: entity.PersonnelNumber,
	}
	if entity.Person != nil {
		profile.FirstName = entity.Person.FirstName
		profile.LastName = entity.Person.LastName
		profile.Birthday = publicOptionalDate(entity.Person.Birthday)
		profile.AccountID = entity.Person.AccountID
	}
	return profile
}

func (c staffDirectoryCapability) PersonByAccountID(ctx context.Context, accountID int64) (*workforce.Person, error) {
	person, err := c.people.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return publicPerson(person), nil
}

func (c staffDirectoryCapability) StaffByPersonID(ctx context.Context, personID int64) (*workforce.StaffProfile, error) {
	staff, err := c.people.GetStaffByPersonID(ctx, personID)
	if err != nil {
		return nil, err
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) StaffByID(ctx context.Context, staffID int64) (*workforce.StaffProfile, error) {
	staff, err := c.people.GetStaffByID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error) {
	return c.people.ResolveStaffIDByAccountID(ctx, accountID)
}
