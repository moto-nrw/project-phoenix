// Package compose binds Reminder Delivery to the shared runtime.
package compose

import (
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// NewQuery constructs the single reminder query provider.
func NewQuery(deps ports.QueryDependencies) reminder.Query {
	return application.NewQuery(deps)
}
