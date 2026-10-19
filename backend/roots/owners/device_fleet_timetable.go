package owners

import (
	"context"
	"time"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func FleetTemplateReader(catalog timetable.TemplateCatalog) interface {
	ListActivityTemplates(context.Context) ([]devicefleetCompose.ActivityTemplate, error)
} {
	return fleetTemplateReader{catalog: catalog}
}

type fleetTemplateReader struct{ catalog timetable.TemplateCatalog }

func (r fleetTemplateReader) ListActivityTemplates(ctx context.Context) ([]devicefleetCompose.ActivityTemplate, error) {
	groups, err := r.catalog.ListTemplateGroups(ctx)
	if err != nil {
		return nil, err
	}
	templates := make([]devicefleetCompose.ActivityTemplate, 0, len(groups))
	for _, group := range groups {
		template := devicefleetCompose.ActivityTemplate{ID: group.ID, Name: group.Name, ParticipantLimit: group.ParticipantLimit()}
		if group.Category != nil {
			name := group.Category.Name
			template.CategoryName = &name
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func FleetInstanceReader(records timetable.SessionRecords) interface {
	ListPlannedActivities(context.Context, time.Time) ([]devicefleetCompose.PlannedActivity, error)
} {
	return fleetInstanceReader{records: records}
}

type fleetInstanceReader struct{ records timetable.SessionRecords }

func (r fleetInstanceReader) ListPlannedActivities(ctx context.Context, today time.Time) ([]devicefleetCompose.PlannedActivity, error) {
	instances, err := r.records.ListSessionInstances(ctx, calendar.DateFromTime(today))
	if err != nil {
		return nil, err
	}
	planned := make([]devicefleetCompose.PlannedActivity, 0, len(instances))
	for _, instance := range instances {
		planned = append(planned, devicefleetCompose.PlannedActivity{
			ID: instance.ID, Title: instance.Title, RoomID: instance.RoomID,
			ActivityGroupID: instance.ActivityGroupID, ActiveGroupID: instance.ActiveGroupID,
			Planned:        timetable.IsPlannedInstanceStatus(instance.Status),
			StartWallClock: calendar.NormalizeWallClock(instance.StartTime),
		})
	}
	return planned, nil
}
