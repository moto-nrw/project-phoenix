package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// DashboardDependencies are the legacy readers the dashboard aggregate needs.
type DashboardDependencies struct {
	ActiveGroups   studentpresence.SessionRecords
	Templates      application.TemplateSource
	Instances      application.PlannedActivitySource
	PickupSchedule careplan.BulkPickupTimes
}

func NewDashboardSources(deps DashboardDependencies) DashboardSources {
	if deps.ActiveGroups == nil || deps.Templates == nil || deps.Instances == nil || deps.PickupSchedule == nil {
		panic("devicefleet dashboard sources: all dependencies are required")
	}
	return application.DashboardSources{TemplateSource: deps.Templates, PlannedActivitySource: deps.Instances, Sessions: deps.ActiveGroups, Pickups: deps.PickupSchedule}
}
