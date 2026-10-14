package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// WorkPresenceQuery is today's work status map, the one read the reminder
// scheduler and the staff recipient resolver take from the work sessions.
type WorkPresenceQuery interface {
	GetTodayPresenceMap(context.Context) (map[int64]string, error)
}

type workPresence struct{ capability workforce.Capability }

// WorkPresence binds GetTodayPresenceMap to the Workforce capability.
func WorkPresence(capability workforce.Capability) WorkPresenceQuery {
	return workPresence{capability: capability}
}

func (p workPresence) GetTodayPresenceMap(ctx context.Context) (map[int64]string, error) {
	return p.capability.WorkPresenceMap(ctx)
}
