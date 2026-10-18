package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WorkforceAdminCapabilities bundles the public Workforce contracts the staff
// administration and time-tracking resources consume (#2690).
type WorkforceAdminCapabilities struct {
	Staff              workforceModule.StaffDirectory
	Documents          workforceModule.StaffDocuments
	WorkSessions       workforceModule.WorkSessions
	StaffAbsences      workforceModule.StaffAbsences
	WorkTimeMonth      workforceModule.WorkTimeMonths
	BalanceAdjustments workforceModule.BalanceAdjustments
	MonthClosing       workforceModule.MonthClosing
	Overview           workforceModule.StaffOverview
	AuditLog           workforceModule.TimeTrackingAuditLog
	TimeExport         workforceModule.StaffTimeExport
}

// NewWorkforceAdminCapabilities bundles the staff directory and the
// personnel-record administration with the time-tracking contracts the
// Workforce composition built. A nil time-tracking side leaves those
// contracts nil, so a partial composition still serves the routes it has.
func NewWorkforceAdminCapabilities(people peoplePorts.PersonDirectory, staffAdmin *workforceModule.StaffAdmin, timeTracking *workforceCompose.TimeTracking) WorkforceAdminCapabilities {
	capabilities := WorkforceAdminCapabilities{Staff: newStaffDirectoryCapability(people, staffAdmin)}
	if staffAdmin != nil {
		capabilities.Documents = staffAdmin
	}
	if timeTracking != nil {
		capabilities.WorkSessions = timeTracking.WorkSessions
		capabilities.StaffAbsences = timeTracking.StaffAbsences
		capabilities.WorkTimeMonth = timeTracking.WorkTimeMonths
		capabilities.BalanceAdjustments = timeTracking.BalanceAdjustments
		capabilities.MonthClosing = timeTracking.MonthClosing
		capabilities.Overview = timeTracking.Overview
		capabilities.AuditLog = timeTracking.AuditLog
		capabilities.TimeExport = timeTracking.TimeExport
	}
	return capabilities
}

// timeTrackingAccountStartDate resolves the tenant's time-account start day
// for the time-tracking config endpoint; the setting key stays next to its
// registry definition. A nil settings service leaves the start date unset.
func timeTrackingAccountStartDate(settings settingsCompose.SettingsService) func(context.Context) (string, error) {
	if settings == nil {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		return settings.ResolveString(ctx, configModels.KeyTimeTrackingAccountStartDate)
	}
}

// staffDirectoryCapability serves the staff lookups of the public Workforce
// personnel-record contract from the retained person service (#2690).
// Lookups pass their failures through unchanged so the not-found
// classification of the HTTP layer keeps working. The personnel-record
// administration itself is Workforce's own (workforce.StaffAdmin, #3752) and
// is embedded unchanged.
type staffDirectoryCapability struct {
	workforceModule.StaffRecordAdmin
	people staffRecordPeople
}

// staffRecordPeople are the retained person service's staff lookups the
// personnel-record contract serves.
type staffRecordPeople interface {
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*peopleValues.Staff, error)
	GetStaffByID(ctx context.Context, staffID int64) (*peopleValues.Staff, error)
	ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error)
}

// newStaffDirectoryCapability serves workforce.StaffDirectory: the lookups
// from the retained person service, the administration from Workforce.
func newStaffDirectoryCapability(people peoplePorts.PersonDirectory, admin workforceModule.StaffRecordAdmin) workforceModule.StaffDirectory {
	if people == nil || admin == nil {
		panic("staff directory capability: person service and personnel-record administration are required")
	}
	return staffDirectoryCapability{StaffRecordAdmin: admin, people: people}
}

func (c staffDirectoryCapability) PersonByAccountID(ctx context.Context, accountID int64) (*workforceModule.Person, error) {
	person, err := c.people.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return publicPerson(person), nil
}

func (c staffDirectoryCapability) StaffByPersonID(ctx context.Context, personID int64) (*workforceModule.StaffProfile, error) {
	staff, err := c.people.GetStaffByPersonID(ctx, personID)
	if err != nil {
		return nil, err
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) StaffByID(ctx context.Context, staffID int64) (*workforceModule.StaffProfile, error) {
	staff, err := c.people.GetStaffByID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return publicStaffProfile(staff), nil
}

func (c staffDirectoryCapability) ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error) {
	return c.people.ResolveStaffIDByAccountID(ctx, accountID)
}

func publicPerson(entity *peopleValues.DirectoryPersonSnapshot) *workforceModule.Person {
	if entity == nil {
		return nil
	}
	return &workforceModule.Person{
		ID: entity.ID, FirstName: entity.FirstName, LastName: entity.LastName, Birthday: optionalDateString(entity.Birthday), AccountID: entity.AccountID,
	}
}

// publicStaffProfile is the public staff profile of a retained row, with the
// person's fields when the row was read with it.
func publicStaffProfile(entity *peopleValues.Staff) *workforceModule.StaffProfile {
	if entity == nil {
		return nil
	}
	profile := &workforceModule.StaffProfile{
		ID: entity.ID, PersonID: entity.PersonID, EmploymentType: entity.EmploymentType, WorkTimeModelID: entity.WorkTimeModelID,
		RotationAnchorDate: optionalDateString(entity.RotationAnchorDate), PersonnelNumber: entity.PersonnelNumber,
	}
	if entity.Person != nil {
		profile.FirstName = entity.Person.FirstName
		profile.LastName = entity.Person.LastName
		profile.Birthday = optionalDateString(entity.Person.Birthday)
		profile.AccountID = entity.Person.AccountID
	}
	return profile
}

func optionalDateString(value *timezone.Date) string {
	if value == nil {
		return ""
	}
	return value.String()
}
