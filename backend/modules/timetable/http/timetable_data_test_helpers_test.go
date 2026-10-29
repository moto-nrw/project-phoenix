package timetablehttp

import (
	"testing"

	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/uptrace/bun"
)

// testTimetable bundles what the composition root hands the timetable routes
// from the Timetable owner: the template writes (embedded, so the value serves
// Dependencies.Templates directly), the planner reads, the conflict
// detection and the retained attendance correction.
type testTimetable struct {
	timetableModule.TemplateAdministration
	data        timetableModule.TimetableDataCapability
	conflicts   timetableModule.ConflictDetectionCapability
	corrections timetableModule.AttendanceCorrections
	lock        timetableModule.RecurrenceWriteLock
	review      timetableModule.TemplateRosterReview
	// materialization is the composed one unless the suite replaced it.
	materialization timetableModule.MaterializationCapability
}

func (t *testTimetable) RosterReview() timetableModule.TemplateRosterReview { return t.review }

func (t *testTimetable) TimetableData() timetableModule.TimetableDataCapability { return t.data }

func (t *testTimetable) ConflictDetection() timetableModule.ConflictDetectionCapability {
	return t.conflicts
}

func (t *testTimetable) AttendanceCorrections() timetableModule.AttendanceCorrections {
	return t.corrections
}

func (t *testTimetable) RecurrenceLock() timetableModule.RecurrenceWriteLock { return t.lock }

// testTimetableOptions are the collaborators a suite replaces: Enrollment's
// care-offering checks and roster resync, the materialization (nil = the
// owner's real engine over the test database) and the instance lifecycle
// whose deviation machinery a split preserves (nil = no deviations).
type testTimetableOptions struct {
	validateCareOfferingSeries func(context.Context, int64) error
	validateOfferingSource     func(context.Context, []int64, []int64, *int64) error
	resyncOfferingRoster       func(context.Context, timetableModule.OfferingRosterResyncInput) error
	materialization            timetableModule.MaterializationCapability
	nonWorkingDays             timetableModule.NonWorkingDayCalendar
	instances                  timetableModule.InstanceLifecycleCapability
}

// testTimetableData builds the Timetable owner's template writes, planner
// reads (TimetableData()) and conflict detection (ConflictDetection())
// against the test database — the test-side equivalent of the factory
// wiring.
func testTimetableData(db *bun.DB, clocks ...func() time.Time) *testTimetable {
	return testTimetableDataWithCareValidator(db, nil, clocks...)
}

func testTimetableDataWithCareValidator(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	clocks ...func() time.Time,
) *testTimetable {
	return testTimetableDataWithOfferingCallbacks(db, validateCareOfferingSeries, nil, nil, clocks...)
}

func testTimetableDataWithOfferingCallbacks(
	db *bun.DB,
	validateCareOfferingSeries func(context.Context, int64) error,
	validateOfferingSource func(context.Context, []int64, []int64, *int64) error,
	resyncOfferingRoster func(context.Context, timetableModule.OfferingRosterResyncInput) error,
	clocks ...func() time.Time,
) *testTimetable {
	return testTimetableWith(db, testTimetableOptions{
		validateCareOfferingSeries: validateCareOfferingSeries,
		validateOfferingSource:     validateOfferingSource,
		resyncOfferingRoster:       resyncOfferingRoster,
	}, clocks...)
}

// testTimetableWith composes the owner through the root's test composition,
// with the suite's collaborators.
func testTimetableWith(db *bun.DB, options testTimetableOptions, clocks ...func() time.Time) *testTimetable {
	composed := TimetableOptions{
		ValidateCareOfferingSeries: options.validateCareOfferingSeries,
		ValidateOfferingSource:     options.validateOfferingSource,
		ResyncOfferingRoster:       options.resyncOfferingRoster,
		Materialization:            options.materialization,
		NonWorkingDays:             options.nonWorkingDays,
		Instances:                  options.instances,
	}
	if len(clocks) > 0 && clocks[0] != nil {
		composed.Clock = clocks[0]
	}
	module, err := RootCompositions.TimetableHTTP(db, composed)
	if err != nil {
		panic(err)
	}
	return &testTimetable{
		TemplateAdministration: module.Templates,
		data:                   module.Data,
		conflicts:              module.ConflictDetection,
		lock:                   module.RecurrenceLock,
		corrections:            module.AttendanceCorrections,
		review:                 module.RosterReview,
		materialization:        module.Materialization,
	}
}

// TimetableOptions are the collaborators a suite replaces in the root's
// Timetable composition: Enrollment's care-offering checks and roster resync,
// the materialization, the instance lifecycle and the clock.
type TimetableOptions struct {
	ValidateCareOfferingSeries func(context.Context, int64) error
	ValidateOfferingSource     func(context.Context, []int64, []int64, *int64) error
	ResyncOfferingRoster       func(context.Context, timetableModule.OfferingRosterResyncInput) error
	Materialization            timetableModule.MaterializationCapability
	NonWorkingDays             timetableModule.NonWorkingDayCalendar
	Instances                  timetableModule.InstanceLifecycleCapability
	Clock                      func() time.Time
}

// TimetableHTTPModule is what the root hands the timetable routes from the
// owner: the template writes, the planner reads, the conflict detection, the
// attendance correction and the recurrence gate.
type TimetableHTTPModule struct {
	Templates             timetableModule.TemplateAdministration
	RosterReview          timetableModule.TemplateRosterReview
	Data                  timetableModule.TimetableDataCapability
	ConflictDetection     timetableModule.ConflictDetectionCapability
	AttendanceCorrections timetableModule.AttendanceCorrections
	RecurrenceLock        timetableModule.RecurrenceWriteLock
	Materialization       timetableModule.MaterializationCapability
}

// TimetableModule is what the route suites take from the root's timetable
// graph: the instance lifecycle and the owner's planning surfaces.
type TimetableModule struct {
	Instance          timetableModule.InstanceLifecycleCapability
	Templates         timetableModule.TemplateAdministration
	Data              timetableModule.TimetableDataCapability
	Deviations        timetableModule.StaffDeviations
	ConflictDetection timetableModule.ConflictDetectionCapability
}

// TimetableRows are the retained repositories the composition root binds to
// the Timetable owner, with the school calendar's periods, their usage and the
// room store the suites arrange through.
type TimetableRows struct {
	timetableCompose.Repositories
	Timetable timetableModule.Capability
	ShiftType timetablePorts.WorkforceShiftTypeRows
	Room      interface {
		Delete(ctx context.Context, rawID any) error
	}
	CalendarPeriods CalendarPeriods
	PeriodUsage     timetablePorts.CalendarPeriodUsageRepository
}

// SchoolCalendar serves the calendar periods the template routes read.
func (r TimetableRows) SchoolCalendar() CalendarPeriods { return r.CalendarPeriods }

// CalendarPeriodUsage counts the planning owners' references per period.
func (r TimetableRows) CalendarPeriodUsage() timetablePorts.CalendarPeriodUsageRepository {
	return r.PeriodUsage
}

// SessionRecords are Student Presence's live sessions and supervisions the
// deviation suites arrange and read back.
type SessionRecords interface {
	CreateSession(ctx context.Context, group *studentpresence.LiveGroup) error
	CreateSupervision(ctx context.Context, supervision *studentpresence.GroupSupervision) error
	FindActiveByStaffID(ctx context.Context, staffID int64) ([]*studentpresence.GroupSupervision, error)
}

// RootCompositions are the typed root's compositions the internal suites run
// over. The root imports this package, so these internal tests cannot import
// it; root_composition_test.go (package timetablehttp_test) binds them.
var RootCompositions struct {
	// Module composes the timetable graph over a fresh test database.
	Module func(t *testing.T, clocks ...func() time.Time) (*bun.DB, TimetableModule)
	// TimetableHTTP composes the owner the timetable routes drive over db.
	TimetableHTTP func(db *bun.DB, options TimetableOptions) (TimetableHTTPModule, error)
	// Rows composes the retained repositories over db.
	Rows func(db *bun.DB, clocks ...func() time.Time) (TimetableRows, error)
	// People serves the People port of the timetable routes over db.
	People func(db *bun.DB) (People, error)
	// RecurrenceLock is the owner's tenant recurrence gate over db.
	RecurrenceLock func(db *bun.DB) (timetableModule.RecurrenceWriteLock, error)
	// SessionRecords are Student Presence's live sessions over db.
	SessionRecords func(db *bun.DB) SessionRecords
	// OfferingSources serve the offering-source support of the routes over db.
	OfferingSources func(t *testing.T, db *bun.DB) (timetableModule.OfferingSourceSupport, error)
	// StaffAccountPeople is the operations' person port for an acting account
	// that is this person and staff member.
	StaffAccountPeople func(personID, staffID int64, session timetablePorts.RequestSession) interface {
		timetablePorts.OperationPeople
		timetablePorts.OperationCaller
	}
	// ErrCareOfferingInvalid is what Care Plan's care-offering checks return
	// for an offering a template change would invalidate.
	ErrCareOfferingInvalid error
}

// must returns a composed fixture or panics on its error, as the suites'
// helpers always have.
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

// mustComposed turns a fixture constructor into one that panics on its
// error, as the suites' helpers always have.
func mustComposed[T any](compose func(*bun.DB, ...func() time.Time) (T, error)) func(*bun.DB, ...func() time.Time) T {
	return func(db *bun.DB, clocks ...func() time.Time) T {
		value, err := compose(db, clocks...)
		if err != nil {
			panic(err)
		}
		return value
	}
}

// mustTimetableTestRepositories composes the retained repositories the
// composition root binds to the Timetable owner.
var mustTimetableTestRepositories = mustComposed(func(db *bun.DB, clocks ...func() time.Time) (TimetableRows, error) {
	return RootCompositions.Rows(db, clocks...)
})

// BoundTimetableRepositories hands the external suites the retained
// repositories the composition root binds to the Timetable owner, so their
// arrangements and assertions go through the live adapters.
var BoundTimetableRepositories = mustTimetableTestRepositories

// calendarPeriodUsageFor serves the usage port from the planning owners'
// per-period reference counts, converting by shape.
func calendarPeriodUsageFor(usage timetablePorts.CalendarPeriodUsageRepository) CalendarPeriodUsage {
	return usageFunc(func(ctx context.Context) (map[int64]CalendarPeriodUsageCounts, error) {
		values, err := usage.Usage(ctx)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]CalendarPeriodUsageCounts, len(values))
		for id, value := range values {
			result[id] = CalendarPeriodUsageCounts(value)
		}
		return result, nil
	})
}

type usageFunc func(context.Context) (map[int64]CalendarPeriodUsageCounts, error)

func (f usageFunc) UsageCounts(ctx context.Context) (map[int64]CalendarPeriodUsageCounts, error) {
	return f(ctx)
}
