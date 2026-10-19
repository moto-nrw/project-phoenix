package devicescanbinding

import (
	"context"

	scanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar" // groupDirectory binds the education group lookup of the daily-checkout
	// gate to the retained education service.
)

type groupDirectory struct{ education scanCompose.SchoolGroups }

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
type pickupPlan struct{ pickups scanCompose.PickupReader }

func (p pickupPlan) Effective(ctx context.Context, studentID int64, day calendar.Date) (*ports.Pickup, error) {
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
