package compose

import (
	"context"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// NewCommand binds the independent tenant transaction and synchronous post-commit dispatch.
func NewCommand(runtime ports.CommandRuntime, deps ports.DeliveryDependencies) reminder.Command {
	return application.NewCommand(runtime, NewPreparation(runtime, deps))
}

// NewPushDelivery binds reminder revalidation to the root's tenant runtime.
func NewPushDelivery(runtime ports.CommandRuntime, deps ports.DeliveryDependencies) func(context.Context, []byte) error {
	return application.NewPushDelivery(deps, runtime)
}

// NewPreparation exposes the transactional half to roots that supply their
// own commit boundary; its continuation still sends through the same outbox.
func NewPreparation(runtime ports.CommandRuntime, deps ports.DeliveryDependencies) ports.GuardianPreparation {
	return application.NewPreparation(deps, runtime)
}
