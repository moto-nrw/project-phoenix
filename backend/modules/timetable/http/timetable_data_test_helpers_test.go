package timetablehttp

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
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
}

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

// testTimetableWith composes the owner through the shared test composition
// (services.NewTimetableHTTPTestModule), with the suite's collaborators.
func testTimetableWith(db *bun.DB, options testTimetableOptions, clocks ...func() time.Time) *testTimetable {
	composed := []testutil.TimetableHTTPOption{
		testutil.WithCareOfferingSeriesValidator(options.validateCareOfferingSeries),
		testutil.WithOfferingSourceValidator(options.validateOfferingSource),
		testutil.WithOfferingRosterResync(options.resyncOfferingRoster),
		testutil.WithTimetableMaterialization(options.materialization),
		testutil.WithTimetableInstances(options.instances),
	}
	if len(clocks) > 0 && clocks[0] != nil {
		composed = append(composed, testutil.WithTimetableClock(clocks[0]))
	}
	module, err := testutil.NewTimetableHTTP(db, composed...)
	if err != nil {
		panic(err)
	}
	return &testTimetable{
		TemplateAdministration: module.Templates,
		data:                   module.Data,
		conflicts:              module.ConflictDetection,
		lock:                   module.RecurrenceLock,
		corrections:            module.AttendanceCorrections,
	}
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
var mustTimetableTestRepositories = mustComposed(testutil.NewTimetableRepositories)

// BoundTimetableRepositories hands the external suites the retained
// repositories the composition root binds to the Timetable owner, so their
// arrangements and assertions go through the live adapters.
var BoundTimetableRepositories = mustTimetableTestRepositories

// calendarPeriodUsageFor serves the usage port from the planning owners'
// per-period reference counts, converting by shape.
func calendarPeriodUsageFor(usage *timetableCompose.CalendarPeriodUsageRepository) CalendarPeriodUsage {
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
