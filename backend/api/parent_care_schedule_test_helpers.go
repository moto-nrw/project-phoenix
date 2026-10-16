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

func NewParentCareScheduleTestService(db transactionruntime.Database, module StudentTestModule) (*parentService.Portal, error) {
	repos, err := NewStudentTestRepositories(db, module.Audit)
	if err != nil {
		return nil, err
	}
	parents, err := NewParentRouteTestRepositories(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	return parentportalcompose.New(parentportalcompose.Dependencies{
		ChildRepo: parents.ParentChildReads(), StudentRepo: repos.Student, PersonRepo: repos.Person,
		StudentGuardianRepo: parents.StudentGuardian, GuardianProfileRepo: parents.GuardianProfile,
		Settings: BindParentSettings(module.Settings), ArrivalSchedules: module.ArrivalSchedule, PickupSchedules: module.PickupSchedule,
		CareRequests: module.CareRequests, CareRequestRepo: NewParentCareRequestReads(repos.CarePlan),
		FamilyProtectionEvents: repos.FamilyProtection, ParentRequestShares: repos.ParentRequestShare,
		StatusDayRepo: repos.StudentStatusDay, MessageThreadRepo: repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		Logger:      slog.Default(),
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
