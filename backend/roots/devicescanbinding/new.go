package devicescanbinding

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	scanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func New(deps scanCompose.Dependencies) scanCompose.DeviceScan {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	clock := clock{now: now}
	var groups ports.Groups
	if deps.Education != nil {
		groups = groupDirectory{education: deps.Education}
	}
	var pickups ports.Pickups
	if deps.Pickups != nil {
		pickups = pickupPlan{pickups: deps.Pickups}
	}
	return scanCompose.New(deps, scanCompose.Bindings{Visits: visits{Presence: deps.Active, active: deps.Active, clock: clock}, Sessions: sessions{Presence: deps.Active, active: deps.Active, presence: deps.Presence, clock: clock}, Attendance: attendance{active: deps.Active}, Groups: groups, Pickups: pickups, Clock: clock})
}

type clock struct{ now func() time.Time }

func (c clock) Now() time.Time                      { return c.now() }
func (c clock) Day(instant time.Time) calendar.Date { return calendar.DateFromTime(instant) }
func NewSessionLifecycle(active studentpresence.Presence, counts OpenVisitCounts, presence devicescan.SupervisionQuery, people scanCompose.PersonDirectory, heartbeat interface {
	PingDevice(context.Context, string) error
}, mirror scanCompose.SessionMirror, logger *slog.Logger, principal devicescan.DevicePrincipal) devicescan.SessionLifecycle {
	return scanCompose.NewSessionLifecycle(lifecycleStore{OpenVisitCounts: counts, active: active, presence: presence}, scanCompose.NewLifecyclePeople(people), heartbeat, mirror, logger, principal)
}
