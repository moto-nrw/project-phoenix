package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
)

// activityCatalog binds the activity provisioning of the special rooms to
// the retained activity service.
type activityCatalog struct{ activities activitiesSvc.ActivityService }

func activityFromGroup(group *activities.Group) *ports.Activity {
	if group == nil {
		return nil
	}
	return &ports.Activity{
		ID: group.ID, Name: group.Name, MaxParticipants: group.MaxParticipants,
		PlannedRoomID: group.PlannedRoomID, IsSystem: group.IsSystem, IsOpen: group.IsOpen,
	}
}

func (c activityCatalog) Find(ctx context.Context, id int64) (*ports.Activity, error) {
	group, err := c.activities.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	return activityFromGroup(group), nil
}

func (c activityCatalog) ListByName(ctx context.Context, name string) ([]ports.Activity, error) {
	groups, err := c.activities.ListGroups(ctx, &activities.GroupListQuery{Name: name})
	if err != nil {
		return nil, err
	}
	result := make([]ports.Activity, 0, len(groups))
	for _, group := range groups {
		if activity := activityFromGroup(group); activity != nil {
			result = append(result, *activity)
		}
	}
	return result, nil
}

func (c activityCatalog) ListCategories(ctx context.Context) ([]ports.Category, error) {
	categories, err := c.activities.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ports.Category, 0, len(categories))
	for _, category := range categories {
		if category == nil {
			continue
		}
		result = append(result, ports.Category{ID: category.ID, Name: category.Name})
	}
	return result, nil
}

func (c activityCatalog) CreateCategory(ctx context.Context, input ports.NewCategory) (ports.Category, error) {
	created, err := c.activities.CreateCategory(ctx, &activities.Category{
		Name: input.Name, Description: input.Description, Color: input.Color, IsSystem: input.IsSystem,
	})
	if err != nil {
		return ports.Category{}, err
	}
	return ports.Category{ID: created.ID, Name: created.Name}, nil
}

func (c activityCatalog) CreateActivity(ctx context.Context, input ports.NewActivity) (ports.Activity, error) {
	// A system activity carries no supervisors and no schedules; created_by
	// stays unset.
	created, err := c.activities.CreateGroup(ctx, &activities.Group{
		Name: input.Name, MaxParticipants: input.MaxParticipants, IsOpen: input.IsOpen,
		CategoryID: input.CategoryID, PlannedRoomID: input.PlannedRoomID, IsSystem: input.IsSystem,
	}, []int64{}, []*activities.Schedule{})
	if err != nil {
		return ports.Activity{}, err
	}
	return *activityFromGroup(created), nil
}

// groupDirectory binds the education group lookup of the daily-checkout
// gate to the retained education service.
type groupDirectory struct{ education educationSvc.Service }

func (g groupDirectory) Find(ctx context.Context, id int64) (*ports.Group, error) {
	group, err := g.education.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, nil
	}
	return &ports.Group{ID: group.ID, Name: group.Name, RoomID: group.RoomID}, nil
}

// pickupPlan binds the effective pickup lookup to the retained pickup
// schedule service.
type pickupPlan struct{ pickups PickupReader }

func (p pickupPlan) Effective(ctx context.Context, studentID int64, day timezone.Date) (*ports.Pickup, error) {
	effective, err := p.pickups.GetEffectivePickupTimeForDate(ctx, studentID, day)
	if err != nil {
		return nil, err
	}
	if effective == nil {
		return nil, nil
	}
	pickup := &ports.Pickup{Time: effective.PickupTime, Notes: effective.Notes}
	for _, note := range effective.DayNotes {
		pickup.DayNotes = append(pickup.DayNotes, note.Content)
	}
	return pickup, nil
}
