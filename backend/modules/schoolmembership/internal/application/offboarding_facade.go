package application

import "github.com/moto-nrw/project-phoenix/modules/schoolmembership"

func NewOffboardingFacade(service *Service) *schoolmembership.Offboarding {
	offboarding := schoolmembership.NewOffboarding(service.PreviewRetirement, service.RetireStaff, service.EraseNextRetiredStaff,
		schoolmembership.Reinstatement{Staff: service.ReinstateStaff, Teacher: service.ReinstateTeacher})
	offboarding.PlannedExits = NewPlannedExitsFacade(service)
	return offboarding
}
