package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/uptrace/bun"

	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The compositions the timetable route suites (modules/timetable/http) drive
// (#2732), composed in services so the suites import neither the retained
// repositories nor the retained models.

// NewTimetableRepositories composes the retained repositories the timetable
// route suites arrange and assert through.
func NewTimetableRepositories(db *bun.DB, clocks ...func() time.Time) (services.TimetableHTTPTestRows, error) {
	return services.NewTimetableHTTPTestRows(db, clocks...)
}

// ErrCareOfferingInvalid is what Care Plan's care-offering checks return for
// an offering a template change would invalidate; wrap it in a validator to
// make the check refuse.
var ErrCareOfferingInvalid = services.TimetableHTTPTestCareOfferingInvalid

// TimetableHTTPOption replaces one collaborator of the Timetable owner the
// timetable route suites drive.
type TimetableHTTPOption func(*services.TimetableHTTPTestOptions)

// WithCareOfferingSeriesValidator replaces Care Plan's check of a template
// linked to a care offering.
func WithCareOfferingSeriesValidator(validate func(context.Context, int64) error) TimetableHTTPOption {
	return func(options *services.TimetableHTTPTestOptions) { options.ValidateCareOfferingSeries = validate }
}

// WithOfferingSourceValidator replaces Care Plan's check of a template's
// offering sources.
func WithOfferingSourceValidator(validate func(context.Context, []int64, []int64, *int64) error) TimetableHTTPOption {
	return func(options *services.TimetableHTTPTestOptions) { options.ValidateOfferingSource = validate }
}

// WithOfferingRosterResync replaces Enrollment's roster resync of an
// offering-sourced template.
func WithOfferingRosterResync(resync func(context.Context, timetable.OfferingRosterResyncInput) error) TimetableHTTPOption {
	return func(options *services.TimetableHTTPTestOptions) { options.ResyncOfferingRoster = resync }
}

// WithTimetableMaterialization replaces the owner's materialization.
func WithTimetableMaterialization(materialization timetable.MaterializationCapability) TimetableHTTPOption {
	return func(options *services.TimetableHTTPTestOptions) { options.Materialization = materialization }
}

// WithTimetableInstances hands the template split the instance lifecycle
// whose deviations it preserves.
func WithTimetableInstances(instances timetable.InstanceLifecycleCapability) TimetableHTTPOption {
	return func(options *services.TimetableHTTPTestOptions) { options.Instances = instances }
}

// WithTimetableClock replaces the owner's clock.
func WithTimetableClock(clock func() time.Time) TimetableHTTPOption {
	return func(options *services.TimetableHTTPTestOptions) { options.Clock = clock }
}

// NewTimetableHTTP composes the Timetable owner the timetable routes drive:
// template writes, planner reads, conflict detection, attendance correction
// and the recurrence gate.
func NewTimetableHTTP(db *bun.DB, options ...TimetableHTTPOption) (services.TimetableHTTPTestModule, error) {
	var composed services.TimetableHTTPTestOptions
	for _, option := range options {
		option(&composed)
	}
	return services.NewTimetableHTTPTestModule(db, composed)
}

// NewPresenceSessionRecords are Student Presence's live sessions and
// supervisions over the test database.
func NewPresenceSessionRecords(db *bun.DB) *presenceCompose.SessionRecords {
	return services.NewTimetableHTTPTestSessionRecords(db)
}

// NewTimetableRecurrenceLock is the Timetable owner's tenant recurrence gate
// over the test database.
func NewTimetableRecurrenceLock(db *bun.DB) (timetable.RecurrenceWriteLock, error) {
	return services.NewTimetableHTTPTestRecurrenceLock(db)
}

// NewTimetablePeople serves the People port of the timetable routes over the
// test database.
func NewTimetablePeople(db *bun.DB) (services.TimetablePeople, error) {
	return services.NewTimetableHTTPTestPeople(db)
}

// NewTimetableOfferingSources serves the offering-source support of the
// timetable routes from the enrollment decision service over the test
// database.
func NewTimetableOfferingSources(t *testing.T, db *bun.DB) (timetable.OfferingSourceSupport, error) {
	t.Helper()
	return services.NewTimetableHTTPTestOfferingSources(db, testpkg.TenantRuntime(t, db))
}
