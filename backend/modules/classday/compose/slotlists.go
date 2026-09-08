// Package compose wires the class-day read projection (#2701): it hands the
// public owner queries of Timetable & Activities, Student Presence, People
// Directory, School Structure, Facilities and Care Plan to the application and
// binds the consumer-owned ports to the retained schedule services, the
// settings service and the identity chain. The retained services are
// compatibility bindings, not target dependencies; they leave with the
// corresponding owner migrations.
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// CareDayResolver is the retained schedule service's care-day derivation.
type CareDayResolver interface {
	ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]scheduleSvc.CareDayStatus, error)
}

// PickupTimeReader is the retained pickup schedule service's bulk read.
type PickupTimeReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleSvc.EffectivePickupTime, error)
}

// ArrivalTimeReader is the retained arrival schedule service's bulk read.
type ArrivalTimeReader interface {
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleSvc.EffectiveArrivalTime, error)
}

// PickupBaselineReader is the retained date-aware recurring pickup projection
// (no exceptions applied).
type PickupBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to timezone.Date) (*scheduleSvc.PickupBaselineProjection, error)
}

// SettingsReader is the slice of the settings service the lists read.
type SettingsReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	// LockSlotListCutoffPairShared acquires the SHARED advisory lock guarding
	// the Ganztag pickup-cutoff pair so both cutoffs come from one consistent
	// snapshot (#1565 review pass 12).
	LockSlotListCutoffPairShared(ctx context.Context) error
}

// StaffReader resolves the caller's staff record for the read-access verdict.
type StaffReader interface {
	GetCurrentStaff(ctx context.Context) (*userModel.Staff, error)
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
	Settings        SettingsReader
	// UserContext may be nil; the read-access verdict then rests on the
	// admin permission alone.
	UserContext StaffReader
	ListExport  *listexport.RendererService
	Logger      *slog.Logger
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
		settings = settingsBinding{settings: deps.Settings}
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
		Rules:           Rules{},
		Settings:        settings,
		Access:          accessBinding{staff: deps.UserContext},
		ListExport:      deps.ListExport,
		Logger:          deps.Logger,
		Now:             deps.Now,
	})
}

type careDayBinding struct{ resolver CareDayResolver }

func (b careDayBinding) ResolveForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]ports.CareDay, error) {
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

func (b effectiveTimeBinding) PickupTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*time.Time, error) {
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

func (b effectiveTimeBinding) ArrivalTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*time.Time, error) {
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

func (b pickupBaselineBinding) RegularPickupTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]string, error) {
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

type settingsBinding struct{ settings SettingsReader }

func (b settingsBinding) TimetableEnabled(ctx context.Context) (bool, error) {
	return b.settings.ResolveBool(ctx, configModel.KeyTimetableEnabled)
}

// PickupCutoffs resolves both Ganztag cutoffs under the SHARED variant of the
// advisory lock the settings writer takes exclusively. Without it the two
// reads run as separate statements under READ COMMITTED, and an administrator
// lowering BOTH boundaries through two individually valid writes could commit
// between them and hand the reader an inverted pair (#1565 review pass 12).
func (b settingsBinding) PickupCutoffs(ctx context.Context) (string, string, error) {
	if err := b.settings.LockSlotListCutoffPairShared(ctx); err != nil {
		return "", "", err
	}
	short, err := b.settings.ResolveString(ctx, configModel.KeySlotListShortDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve short-day pickup cutoff: %w", err)
	}
	long, err := b.settings.ResolveString(ctx, configModel.KeySlotListLongDayCutoff)
	if err != nil {
		return "", "", fmt.Errorf("resolve long-day pickup cutoff: %w", err)
	}
	return short, long, nil
}

type accessBinding struct{ staff StaffReader }

// CanReadStudents grants administrators and accounts with a staff record.
// Only the legitimate restrictive conditions (no staff profile) collapse to
// "no"; operational failures propagate.
func (b accessBinding) CanReadStudents(ctx context.Context) (bool, error) {
	if authorize.HasPermission("admin:*", jwt.PermissionsFromCtx(ctx)) {
		return true, nil
	}
	if b.staff == nil {
		return false, nil
	}
	staff, err := b.staff.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, usercontext.ErrUserNotLinkedToStaff) {
			return false, nil
		}
		return false, fmt.Errorf("resolve current staff: %w", err)
	}
	return staff != nil, nil
}

// Rules forwards the pure care-plan rules to their owners so every reader of
// the roster shares one derivation (#1565, #2606).
type Rules struct{}

func (Rules) RowCareDay(instanceCompleted bool, row ports.RosterFacts, planVerdict ports.CareDay) ports.CareDay {
	model := &scheduleModel.InstanceStudent{
		Status: row.Status, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
	return ports.CareDay(scheduleSvc.AttendanceRowCareDay(instanceCompleted, model, scheduleSvc.CareDayStatus(planVerdict)))
}

func (Rules) EnrolledOn(student ports.StudentFacts, date, today timezone.Date) bool {
	return userModel.EnrolledOn(&userModel.Student{
		Status: userModel.StudentStatus(student.Status), EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
	}, date, today)
}
