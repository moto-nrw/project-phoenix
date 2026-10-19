//go:build testhooks

package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// NewParentCareScheduleTestService composes the parent portal over the
// student graph's Care Plan side and the live graph it runs beside.
func NewParentCareScheduleTestService(db transactionruntime.Database, module StudentTestModule, live ActiveTestModule, liveSettings ActiveSettings) (*parentService.Portal, error) {
	students, repos, err := NewStudentTestRepositories(db, nil)
	if err != nil {
		return nil, err
	}
	reposCare := NewEnrollmentCarePlanRows(db)
	studentPeople, err := NewStudentPeopleRows(db, owners.MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	messages := NewStudentMessageRows(db)
	members := owners.MustNewMembershipTestRepositories(db)
	parents, err := NewParentRouteTestRepositories(db, members)
	if err != nil {
		return nil, err
	}
	return parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         ParentPortalRuntime{},
		ChildRepo:       parents.ParentChildReads(), StudentRepo: owners.NewParentStudentReads(repos.Student), PersonRepo: owners.NewParentPersonReads(repos.Person),
		StudentGuardianRepo: owners.NewParentStudentGuardianReads(members.StudentGuardian), GuardianProfileRepo: owners.NewParentGuardianProfileReads(members.GuardianProfile),
		Settings:         owners.BindParentSettings(liveSettings.Settings),
		ArrivalSchedules: owners.NewParentArrivalSchedules(live.ArrivalSchedule),
		PickupSchedules:  owners.NewParentPickupSchedules(live.PickupSchedule),
		CareRequests:     module.CareRequests, CareRequestRepo: owners.NewParentCareRequestReads(students.CarePlan),
		FamilyProtectionEvents: owners.BindParentFamilyProtectionReads(studentPeople.FamilyProtection), ParentRequestShares: owners.BindParentRequestShareRecords(studentPeople.ParentRequestShare),
		StatusDayRepo: reposCare.StudentStatusDay, MessageThreadRepo: owners.BindParentMessageThreadAccess(messages.ParentMessageThread),
		MessageRepo: owners.BindParentMessageReads(messages.ParentMessage),
		Logger:      slog.Default(),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	}), nil
}

// ParentPortalCareProfiles is Care Plan's care-profile commands as the
// portal suites hold them without importing Care Plan.
type ParentPortalCareProfiles = careplan.StudentProfileCommands

// NewParentPortalCareProfiles builds Care Plan's care-profile commands the
// guardian portal writes the child's health information and live absence
// flags through, for tests that compose the portal.
func NewParentPortalCareProfiles(db transactionruntime.Database) (ParentPortalCareProfiles, error) {
	return careplanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(careplanCompose.Observation) {})
}
