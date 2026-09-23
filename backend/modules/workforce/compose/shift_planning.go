package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/planning"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

// The readers the Dienstplan side of Workforce needs from other owners. They
// are declared here, at the composition seam, so the planning application
// layer never names a foreign repository contract (#3418).

// StaffOverviewReader is the roster read the week grid needs: every staff
// member of the tenant, plus the named ones an assignment refers to.
type StaffOverviewReader interface {
	ListAllWithPerson(ctx context.Context) ([]*usersModels.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error)
}

// StaffDirectory adds the single lookup a shift write needs, to reject a
// shift for a staff member the tenant does not have. FindByID takes an
// untyped id because the bound repository is the generic one.
type StaffDirectory interface {
	StaffOverviewReader
	FindByID(ctx context.Context, id any) (*usersModels.Staff, error)
}

// CalendarPeriodLookup resolves the School Calendar period a series
// materializes over; the School Calendar capability satisfies it.
type CalendarPeriodLookup = planning.CalendarPeriodLookup

// ActivityInstanceReader loads the timetable blocks the week grid and the
// self-service assignments are built from, in the Timetable owner's
// scheduled-instance vocabulary (the plan with its session state).
type ActivityInstanceReader interface {
	planning.ActivityInstanceRangeReader
	planning.ActivityInstanceBatchReader
}

// InstanceStaffReader loads who is planned into those blocks.
type InstanceStaffReader interface {
	planning.InstanceStaffBatchReader
	planning.AssignmentInstanceStaffReader
}

// RoomReader resolves the "Ort" of an assignment.
type RoomReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModels.Room, error)
}

// ActivityGroupReader resolves the Angebot an assignment belongs to.
type ActivityGroupReader = planning.ActivityGroupBatchReader

// StaffWorkScheduleReader and WorkTimeModelReader feed the contractual
// weekly target of the week grid's summaries.
type StaffWorkScheduleReader interface {
	FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to configModels.CalendarDate) ([]*configModels.StaffWorkSchedule, error)
}

type WorkTimeModelReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*configModels.WorkTimeModel, error)
}

// StaffScheduleOverviewDependencies are the narrow readers the Dienstplan
// week grid is built from.
type StaffScheduleOverviewDependencies struct {
	// Shifts serves the shift rows and the weeks the Dienstplan is in use.
	// NewShiftPlanning binds the Workforce capability; a caller with a
	// narrower transaction-scoped read (the plan export) binds its own.
	Shifts        StaffShiftSource
	Instances     ActivityInstanceReader
	InstanceStaff InstanceStaffReader
	Rooms         RoomReader
	Staff         StaffOverviewReader
	// WorkSchedules and WorkModels are optional: a nil reader degrades to
	// planned-minutes-only summaries (TargetMinutes stays nil).
	WorkSchedules StaffWorkScheduleReader
	WorkModels    WorkTimeModelReader
	// Holidays reduces the weekly targets by the non-working-day Soll (#1418
	// 3a/3b), bound to the School Calendar. Optional: nil skips the reduction.
	Holidays planning.HolidayDatesReader
}

// newStaffScheduleOverview builds the public query and the getter the
// planning facade keeps for its own export path over one service.
func newStaffScheduleOverview(deps StaffScheduleOverviewDependencies) (workforce.StaffScheduleOverviewQuery, planning.StaffScheduleOverviewGetter) {
	service := newStaffScheduleOverviewService(deps)
	return planning.NewStaffScheduleOverviewQuery(service), service
}

func newStaffScheduleOverviewService(deps StaffScheduleOverviewDependencies) planning.StaffScheduleOverviewGetter {
	shifts := NewShiftReadRows(deps.Shifts)
	overview := planning.StaffScheduleOverviewDependencies{
		Shifts:        shifts,
		ShiftWeeks:    shifts,
		Instances:     deps.Instances,
		InstanceStaff: deps.InstanceStaff,
		Rooms:         deps.Rooms,
		Staff:         deps.Staff,
		Holidays:      deps.Holidays,
	}
	// A typed nil reader in an interface field is not nil, so only a set
	// reader is forwarded; the overview's own optional handling does the rest.
	if deps.WorkSchedules != nil {
		overview.WorkSchedules = deps.WorkSchedules
	}
	if deps.WorkModels != nil {
		overview.WorkModels = deps.WorkModels
	}
	return planning.NewStaffScheduleOverviewService(overview)
}

// ShiftPlanningDependencies wires the Dienstplan side of Workforce.
type ShiftPlanningDependencies struct {
	// Workforce serves the native shift, series, exception and shift-type rows.
	Workforce workforce.Capability
	Staff     StaffDirectory
	// CalendarPeriods resolves the period a series materializes over.
	CalendarPeriods CalendarPeriodLookup
	// DeviationEvents appends the shift_moved Änderungsprotokoll row (#1884).
	// Optional: nil skips the audit; production binds it.
	DeviationEvents auditModels.DeviationEventRepository
	Instances       ActivityInstanceReader
	InstanceStaff   InstanceStaffReader
	Rooms           RoomReader
	ActivityGroups  ActivityGroupReader
	// WorkSchedules, WorkModels and Holidays refine the week grid's weekly
	// targets; all three are optional.
	WorkSchedules StaffWorkScheduleReader
	WorkModels    WorkTimeModelReader
	Holidays      planning.HolidayDatesReader
	// CategoryLinker syncs the optional Kategorie-Schichtart mapping, whose FK
	// lives with the Timetable owner. Optional: nil leaves mappings untouched.
	CategoryLinker planning.CategoryLinker
	// DB carries the per-staff advisory lock that makes the overlap check safe
	// under concurrency.
	DB *bun.DB
	// Broadcaster and Logger are optional.
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	// Today overrides the calendar-day clock the materializer compares
	// against; nil keeps timezone.TodayDate.
	Today func() timezone.Date
}

// ShiftPlanning is the composed Dienstplan side of Workforce: the public
// contracts the routes and the plan export consume.
type ShiftPlanning struct {
	ShiftTypes  workforce.ShiftTypeAdministration
	Assignments workforce.StaffAssignmentQuery
	Overview    workforce.StaffScheduleOverviewQuery

	shifts   planning.StaffShiftService
	series   planning.StaffShiftSeriesService
	overview planning.StaffScheduleOverviewGetter
}

// NewShiftPlanning composes the shift, series, shift-type, assignment and
// week-grid services over the Workforce capability rows.
func NewShiftPlanning(deps ShiftPlanningDependencies) (*ShiftPlanning, error) {
	switch {
	case deps.Workforce == nil:
		return nil, errors.New("shift planning: the Workforce capability is required")
	case deps.Staff == nil:
		return nil, errors.New("shift planning: the staff directory is required")
	case deps.CalendarPeriods == nil:
		return nil, errors.New("shift planning: the calendar period lookup is required")
	case deps.Instances == nil || deps.InstanceStaff == nil || deps.Rooms == nil:
		return nil, errors.New("shift planning: the instance, assignment and room readers are required")
	case deps.ActivityGroups == nil:
		return nil, errors.New("shift planning: the activity group reader is required")
	case deps.DB == nil:
		return nil, errors.New("shift planning: the database handle is required for the per-staff write lock")
	}

	logger := deps.Logger
	shiftRows := NewShiftRows(deps.Workforce)
	exceptionRows := NewShiftSeriesExceptionRows(deps.Workforce)
	shiftTypes := planning.NewShiftTypeService(NewShiftTypeRows(deps.Workforce), logger)

	shiftOptions := []planning.StaffShiftOption{
		planning.WithStaffShiftSeriesExceptions(exceptionRows),
		planning.WithStaffShiftBroadcaster(deps.Broadcaster),
	}
	if deps.DeviationEvents != nil {
		shiftOptions = append(shiftOptions, planning.WithStaffShiftDeviationEvents(deps.DeviationEvents))
	}
	lockStaffShifts := NewStaffShiftLock(deps.DB)
	shifts := planning.NewStaffShiftService(shiftRows, deps.Staff, shiftTypes, lockStaffShifts, logger, shiftOptions...)

	series := planning.NewStaffShiftSeriesService(
		NewShiftSeriesRows(deps.Workforce), exceptionRows, shiftRows, deps.Staff, planning.SchoolCalendarPeriods(deps.CalendarPeriods),
		shiftTypes, lockStaffShifts, logger, shifts,
		planning.WithStaffShiftSeriesBroadcaster(deps.Broadcaster),
		planning.WithStaffShiftSeriesToday(deps.Today),
	)

	assignments := planning.NewStaffAssignmentService(planning.StaffAssignmentDependencies{
		InstanceStaffRepo:    deps.InstanceStaff,
		ActivityInstanceRepo: deps.Instances,
		RoomRepo:             deps.Rooms,
		ActivityGroupRepo:    deps.ActivityGroups,
	}, logger)

	overviewQuery, overview := newStaffScheduleOverview(StaffScheduleOverviewDependencies{
		Shifts: deps.Workforce, Instances: deps.Instances, InstanceStaff: deps.InstanceStaff,
		Rooms: deps.Rooms, Staff: deps.Staff, WorkSchedules: deps.WorkSchedules, WorkModels: deps.WorkModels,
		Holidays: deps.Holidays,
	})

	return &ShiftPlanning{
		ShiftTypes:  planning.NewShiftTypeAdministration(shiftTypes, deps.CategoryLinker),
		Assignments: planning.NewStaffAssignmentQuery(assignments),
		Overview:    overviewQuery,
		shifts:      shifts,
		series:      series,
		overview:    overview,
	}, nil
}

// Planning binds the public staff-shift facade. planExport may be nil, in
// which case ExportPlan reports that the export is not configured instead of
// panicking; every other route stays available.
func (p *ShiftPlanning) Planning(planExport planexport.Service) workforce.StaffShiftPlanning {
	return planning.NewStaffShiftPlanning(planning.PlanningDependencies{
		Shifts: p.shifts, Series: p.series, Overview: p.overview, PlanExport: planExport,
	})
}
