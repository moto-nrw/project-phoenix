// Package compose wires the class-day read projection (#2701): it hands the
// public owner queries of Timetable & Activities, Student Presence, People
// Directory, School Structure, Facilities and Care Plan to the application and
// binds the consumer-owned ports to the retained schedule services and the
// settings service; the root binds the read access and the renderer. The
// retained services are compatibility bindings, not target dependencies; they
// leave with the corresponding owner migrations.
package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SlotListSettings resolves the timetable flag and the Ganztag pickup
// cutoffs the lists read; the root binds it to the Settings Platform.
type SlotListSettings interface {
	TimetableEnabled(ctx context.Context) (bool, error)
	PickupCutoffs(ctx context.Context) (short, long string, err error)
}

// SlotListDependencies wires the slot lists. The owner facades are the
// tenant-safe reads; the retained services are compatibility bindings.
type SlotListDependencies struct {
	Timetable       application.TimetableReader
	Presence        application.PresenceReader
	CarePlan        application.CarePlanReader
	Students        application.StudentReader
	Persons         application.PersonReader
	Groups          application.GroupReader
	Rooms           application.RoomReader
	CareDays        CareDayResolver
	PickupTimes     PickupTimeReader
	ArrivalTimes    ArrivalTimeReader
	PickupBaselines PickupBaselineReader
	Settings        SlotListSettings
	// Access decides whether the caller reads children by name; the root
	// binds the admin permission and the caller's staff record.
	Access     ports.ReadAccess
	ListExport application.ListRenderer
	EnrolledOn func(status string, enrolledFrom, enrolledUntil *calendar.Date, date, today calendar.Date) bool
	// Now overrides the service clock. Leave nil in production (defaults to
	// time.Now); tests inject a fixed instant for a deterministic weekday.
	Now func() time.Time
}

// NewSlotLists composes the slot list builder. Settings is required (see
// application.NewSlotLists); every other nil dependency surfaces as a "not
// configured" error on the first build.
func NewSlotLists(deps SlotListDependencies) classday.SlotLists {
	var careDays ports.CareDays
	if deps.CareDays != nil {
		careDays = application.CareDayBinding{Resolver: deps.CareDays}
	}
	var effectiveTimes ports.EffectiveTimes
	if deps.PickupTimes != nil && deps.ArrivalTimes != nil {
		effectiveTimes = application.EffectiveTimeBinding{Pickups: deps.PickupTimes, Arrivals: deps.ArrivalTimes}
	}
	var baselines ports.PickupBaselines
	if deps.PickupBaselines != nil {
		baselines = application.PickupBaselineBinding{Baselines: deps.PickupBaselines}
	}
	return application.NewSlotLists(application.SlotListDependencies{
		Timetable:       deps.Timetable,
		Presence:        deps.Presence,
		CarePlan:        deps.CarePlan,
		Students:        deps.Students,
		Persons:         deps.Persons,
		Groups:          deps.Groups,
		Rooms:           deps.Rooms,
		CareDays:        careDays,
		EffectiveTimes:  effectiveTimes,
		PickupBaselines: baselines,
		Rules:           application.Rules(deps.EnrolledOn),
		Settings:        deps.Settings,
		Access:          deps.Access,
		ListExport:      deps.ListExport,
		Now:             deps.Now,
	})
}

// Timetable's stored status, pinned in list_kind_pin_test.go.

// SlotBlock is one block of the day as the slot lists read it: the Timetable
// plan folded with the Student Presence execution (#2762).
type SlotBlock = ports.ActivityInstance

// SlotRosterRow is one roster row of a block with the attendance it carries.
type SlotRosterRow = ports.InstanceStudent

// TimetableReader is the block and roster read seam the slot lists build on;
// the composition root binds it over both owners.
type TimetableReader = ports.TimetableReader

const SlotAttendanceExpected = application.SlotAttendanceExpected
