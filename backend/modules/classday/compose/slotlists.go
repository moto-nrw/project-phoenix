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

	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareDayResolver is the retained schedule service's care-day derivation.
type CareDayResolver interface {
	ResolveForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]careplan.CareDayStatus, error)
}

// PickupTimeReader is the retained pickup schedule service's bulk read.
type PickupTimeReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*careplan.EffectivePickupTime, error)
}

// ArrivalTimeReader is the retained arrival schedule service's bulk read.
type ArrivalTimeReader interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*careplan.EffectiveArrivalTime, error)
}

// PickupBaselineReader is the retained date-aware recurring pickup projection
// (no exceptions applied).
type PickupBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to calendar.Date) (*careplan.PickupBaselineProjection, error)
}

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
	// EnrolledOn is the People Directory's enrollment rule; the root binds it.
	EnrolledOn func(status string, enrolledFrom, enrolledUntil *calendar.Date, date, today calendar.Date) bool
	// Now overrides the service clock. Leave nil in production (defaults to
	// time.Now); tests inject a fixed instant for a deterministic weekday.
	Now func() time.Time
}

// NewSlotLists composes the slot list builder. Settings is required (see
// application.NewSlotLists); every other nil dependency surfaces as a "not
// configured" error on the first build.
func NewSlotLists(deps SlotListDependencies) classday.SlotLists {
	var settings ports.Settings
	if deps.Settings != nil {
		settings = deps.Settings
	}
	var careDays ports.CareDays
	if deps.CareDays != nil {
		careDays = careDayBinding{resolver: deps.CareDays}
	}
	var effectiveTimes ports.EffectiveTimes
	if deps.PickupTimes != nil && deps.ArrivalTimes != nil {
		effectiveTimes = effectiveTimeBinding{pickups: deps.PickupTimes, arrivals: deps.ArrivalTimes}
	}
	var baselines ports.PickupBaselines
	if deps.PickupBaselines != nil {
		baselines = pickupBaselineBinding{baselines: deps.PickupBaselines}
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
		Rules:           Rules(deps.EnrolledOn),
		Settings:        settings,
		Access:          deps.Access,
		ListExport:      deps.ListExport,
		Now:             deps.Now,
	})
}

type careDayBinding struct{ resolver CareDayResolver }

func (b careDayBinding) ResolveForDate(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]ports.CareDay, error) {
	verdicts, err := b.resolver.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]ports.CareDay, len(verdicts))
	for studentID, verdict := range verdicts {
		out[studentID] = ports.CareDay(verdict)
	}
	return out, nil
}

type effectiveTimeBinding struct {
	pickups  PickupTimeReader
	arrivals ArrivalTimeReader
}

func (b effectiveTimeBinding) PickupTimes(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*time.Time, error) {
	effective, err := b.pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*time.Time, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = entry.PickupTime
		}
	}
	return out, nil
}

func (b effectiveTimeBinding) ArrivalTimes(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*time.Time, error) {
	effective, err := b.arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*time.Time, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = entry.ArrivalTime
		}
	}
	return out, nil
}

type pickupBaselineBinding struct{ baselines PickupBaselineReader }

func (b pickupBaselineBinding) RegularPickupTimes(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]string, error) {
	projection, err := b.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(studentIDs))
	for _, studentID := range studentIDs {
		if row := projection.ForDate(studentID, date); row != nil {
			out[studentID] = row.PickupTime.Format("15:04")
		}
	}
	return out, nil
}

// Rules forwards the pure care-plan rules to their owners so every reader of
// the roster shares one derivation (#1565, #2606).
type Rules func(status string, enrolledFrom, enrolledUntil *calendar.Date, date, today calendar.Date) bool

func (Rules) RowCareDay(instanceCompleted bool, row ports.RosterFacts, planVerdict ports.CareDay) ports.CareDay {
	model := &careplan.CareDayAttendance{Expected: row.Status == scheduleModel.AttendanceStatusExpected, NotScheduled: row.NotScheduled, ManuallyDecided: row.ManualStatusAt != nil, PlanOwnedAbsence: row.StudentStatusDayID != nil || row.PickupExceptionID != nil}
	return ports.CareDay(careplan.AttendanceRowCareDay(instanceCompleted, model, careplan.CareDayStatus(planVerdict)))
}

func (r Rules) EnrolledOn(student ports.StudentFacts, date, today calendar.Date) bool {
	return r(student.Status, student.EnrolledFrom, student.EnrolledUntil, date, today)
}

// SlotBlock is one block of the day as the slot lists read it: the Timetable
// plan folded with the Student Presence execution (#2762).
type SlotBlock = ports.ActivityInstance

// SlotRosterRow is one roster row of a block with the attendance it carries.
type SlotRosterRow = ports.InstanceStudent

// TimetableReader is the block and roster read seam the slot lists build on;
// the composition root binds it over both owners.
type TimetableReader = ports.TimetableReader
