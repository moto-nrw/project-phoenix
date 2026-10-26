package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SessionRecords is the native read and command seam for retained session consumers.
type SessionRecords interface {
	FindSessionInstance(context.Context, int64) (*ScheduledInstance, error)
	ListSessionInstances(context.Context, calendar.Date) ([]ScheduledInstance, error)
	ListActiveSessionInstances(context.Context) ([]ScheduledInstance, error)
	// CreateMirroredSessionInstance records an already-running kiosk session and its plan.
	// Timetable owns the mirrored-session defaults: the instance is spontaneous
	// and active. The existing joined adapter also owns the tenant identity.
	CreateMirroredSessionInstance(context.Context, ScheduledInstance) (int64, error)
	AddSessionInstanceStaff(context.Context, int64, int64) error
}

// TemplateCatalog reads templates with their categories using the established listing.
type TemplateCatalog interface {
	ListTemplateGroups(context.Context) ([]Group, error)
}
