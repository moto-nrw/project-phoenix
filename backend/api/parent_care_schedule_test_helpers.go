//go:build testhooks

package api

import (
	"log/slog"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// NewParentCareScheduleTestService composes the parent portal over the
// student graph's Care Plan side and the live graph it runs beside.
func NewParentCareScheduleTestService(db transactionruntime.Database, module StudentTestModule, live ActiveTestModule) (*parentService.Portal, error) {
	students, repos, err := NewStudentTestRepositories(db, nil)
	if err != nil {
		return nil, err
	}
	studentPeople, err := NewStudentPeopleRows(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	messages := NewStudentMessageRows(db)
	members := MustNewMembershipTestRepositories(db)
	parents, err := NewParentRouteTestRepositories(db, members)
	if err != nil {
		return nil, err
	}
	return parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: ParentGuardianChanges(),
		DeparturePolicy: ParentDeparturePolicy(),
		GuardianPolicy:  NewParentGuardianPolicy(),
		Fingerprints:    SecurityFingerprints{},
		Declarations:    ParentDeclarationPolicy(),
		Locales:         ParentPortalLocales(),
		Runtime:         ParentPortalRuntime{},
		ChildRepo:       parents.ParentChildReads(), StudentRepo: NewParentStudentReads(repos.Student), PersonRepo: NewParentPersonReads(repos.Person),
		StudentGuardianRepo: NewParentStudentGuardianReads(members.StudentGuardian), GuardianProfileRepo: NewParentGuardianProfileReads(members.GuardianProfile),
		Settings: BindParentSettings(live.Settings), ArrivalSchedules: NewParentArrivalSchedules(live.ArrivalSchedule), PickupSchedules: NewParentPickupSchedules(live.PickupSchedule),
		CareRequests: module.CareRequests, CareRequestRepo: NewParentCareRequestReads(students.CarePlan),
		FamilyProtectionEvents: BindParentFamilyProtectionReads(studentPeople.FamilyProtection), ParentRequestShares: BindParentRequestShareRecords(studentPeople.ParentRequestShare),
		StatusDayRepo: repos.StudentStatusDay, MessageThreadRepo: BindParentMessageThreadAccess(messages.ParentMessageThread),
		MessageRepo: BindParentMessageReads(messages.ParentMessage),
		Logger:      slog.Default(),

		CarePlanPolicy: NewParentCarePlanPolicy(),
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
