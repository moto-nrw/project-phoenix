// Package compose builds the holiday closure notification over the ports
// the root binds (ADR 0196).
package compose

import (
	"github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification"
	"github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification/internal/application"
)

// Dependencies are the owners' ports and the units.
type Dependencies = application.Dependencies

// New builds the workflow.
func New(dependencies Dependencies) holidayclosurenotification.Notifications {
	return application.New(dependencies)
}
