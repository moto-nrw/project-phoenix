package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/uptrace/bun"
)

func NewParentCareScheduleTestService(db *bun.DB, module StudentTestModule) (*parentService.Portal, error) {
	repos, err := repositories.NewStudentTestRepositories(db, module.Audit)
	if err != nil {
		return nil, err
	}
	parents, err := repositories.NewParentRouteTestRepositories(db)
	if err != nil {
		return nil, err
	}
	return parentportalcompose.New(parentportalcompose.Dependencies{
		ChildRepo: parents.ParentChild, StudentRepo: repos.Student, PersonRepo: repos.Person,
		StudentGuardianRepo: parents.StudentGuardian, GuardianProfileRepo: parents.GuardianProfile,
		Settings: module.Settings, ArrivalSchedules: module.ArrivalSchedule, PickupSchedules: module.PickupSchedule,
		CareRequests: module.CareRequests, CareRequestRepo: repos.CareScheduleChangeRequest,
		FamilyProtectionEvents: repos.FamilyProtection, ParentRequestShares: repos.ParentRequestShare,
		StatusDayRepo: repos.StudentStatusDay, MessageThreadRepo: repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		Logger:      slog.Default(),
	}), nil
}

// NewParentPortalCareProfiles builds Care Plan's care-profile commands the
// guardian portal writes the child's health information and live absence
// flags through, for tests that compose the portal.
func NewParentPortalCareProfiles(db *bun.DB) (careplan.StudentProfileCommands, error) {
	return careplanCompose.NewStudentProfiles(db, func(careplanCompose.Observation) {})
}
