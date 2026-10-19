package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

type SessionLifecycle = devicescan.SessionLifecycle

// NewSessionLifecycle binds manual kiosk controls to the retained presence
// and people services without constructing a legacy service graph.
func NewSessionLifecycle(store application.LifecycleStore, people application.LifecyclePeople, heartbeat application.SessionHeartbeat, mirror application.SessionMirror, logger *slog.Logger, principal devicescan.DevicePrincipal) SessionLifecycle {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	return application.NewSessionLifecycle(store, people, principal, heartbeat, mirror, clock{now: time.Now}, logger)
}
func NewLifecyclePeople(people PersonDirectory) application.LifecyclePeople {
	return lifecyclePeople{people: people}
}

type SupervisionQuery interface{ devicescan.SupervisionQuery }

type lifecyclePeople struct{ people PersonDirectory }

func (p lifecyclePeople) StaffNames(ctx context.Context, ids []int64) (map[int64]ports.Person, error) {
	return application.StaffNamesBinding(ctx, p.people, ids)
}

func SupervisionEnded(endDate *string, today string) bool {
	return application.SupervisionEnded(endDate, today)
}

func LifecycleSupervisorsBinding(ctx context.Context, source devicescan.SupervisionQuery, sessionID int64) ([]ports.LifecycleSupervisor, error) {
	return application.LifecycleSupervisorsBinding(ctx, source, sessionID)
}
