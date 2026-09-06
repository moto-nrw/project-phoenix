package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/uptrace/bun"
)

func NewParentCareScheduleTestService(db *bun.DB, module StudentTestModule) (parentService.Service, error) {
	repos, err := repositories.NewStudentTestRepositories(db, module.Audit)
	if err != nil {
		return nil, err
	}
	parents, err := repositories.NewParentRouteTestRepositories(db)
	if err != nil {
		return nil, err
	}
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo: parents.ParentChild, StudentRepo: repos.Student, PersonRepo: repos.Person,
		StudentGuardianRepo: parents.StudentGuardian, GuardianProfileRepo: parents.GuardianProfile,
		Settings: module.Settings, ArrivalSchedules: module.ArrivalSchedule, PickupSchedules: module.PickupSchedule,
		CareRequests: module.CareRequests, CareRequestRepo: repos.CareScheduleChangeRequest,
		FamilyProtectionEvents: repos.FamilyProtection, ParentRequestShares: repos.ParentRequestShare,
		StatusDayRepo: repos.StudentStatusDay, MessageThreadRepo: repos.ParentMessageThread,
		MessageRepo: repos.ParentMessage,
		DB:          db, Logger: slog.Default(),
	}), nil
}
