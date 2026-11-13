package api

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	settingsplatform "github.com/moto-ogs/project-phoenix/modules/settings"
)

// groupStudentsInClassDay binds the group student list to the day-planning
// owner's rule for how long "Im Unterricht" holds, read with the school's care
// day end like the group live view (#5710).
type groupStudentsInClassDay struct {
	settings careDayEndSettings
	now      func() time.Time
}

type careDayEndSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

func newGroupStudentsInClassDay(settings careDayEndSettings) groupStudentsInClassDay {
	return groupStudentsInClassDay{settings: settings, now: time.Now}
}

func (d groupStudentsInClassDay) InClassStillHolds(ctx context.Context) (bool, error) {
	careDayEnd, err := d.settings.ResolveString(ctx, settingsplatform.KeySessionEndTime)
	if err != nil {
		return false, err
	}
	return careplan.InClassUntilCareDayEnd(careDayEnd, d.now()), nil
}
