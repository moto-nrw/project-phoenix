package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
)

func NewOffboarding(dependencies Dependencies, appendDeletion func(context.Context, workforce.StaffAbsence, int64) error) (*workforce.Offboarding, error) {
	if appendDeletion == nil {
		return nil, errors.New("workforce: absence deletion audit is required")
	}
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return workforce.NewOffboarding(
		service.LockStaffOffboarding,
		service.PreviewStaffOffboarding,
		application.NewOffboardingExecute(service, appendDeletion)), nil
}
