package timetablehttp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupTimetableModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.TimetableTestModule, timetableAPI.CalendarPeriods) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, calendar, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module, calendar
}

// The root imports this package, so the internal suites reach its
// compositions through these bindings.
func init() {
	root := &timetableAPI.RootCompositions
	root.Module = func(t *testing.T, clocks ...func() time.Time) (*bun.DB, timetableAPI.TimetableModule) {
		t.Helper()
		db, module, _ := setupTimetableModule(t, clocks...)
		return db, timetableAPI.TimetableModule{
			Instance: module.Instance, Templates: module.TimetableData.Templates, Data: module.TimetableData.Data,
			Deviations: module.TimetableData.Deviations, ConflictDetection: module.ConflictDetection,
		}
	}
	root.TimetableHTTP = func(db *bun.DB, options timetableAPI.TimetableOptions) (timetableAPI.TimetableHTTPModule, error) {
		composed := []api.TimetableHTTPOption{
			api.WithCareOfferingSeriesValidator(options.ValidateCareOfferingSeries),
			api.WithOfferingSourceValidator(options.ValidateOfferingSource),
			api.WithOfferingRosterResync(options.ResyncOfferingRoster),
			api.WithTimetableMaterialization(options.Materialization),
			api.WithTimetableInstances(options.Instances),
		}
		if options.Clock != nil {
			composed = append(composed, api.WithTimetableClock(options.Clock))
		}
		module, err := api.NewTimetableHTTP(testpkg.MustDatabase(db), composed...)
		if err != nil {
			return timetableAPI.TimetableHTTPModule{}, err
		}
		return timetableAPI.TimetableHTTPModule{
			Templates: module.Templates, Data: module.Data, ConflictDetection: module.ConflictDetection,
			AttendanceCorrections: module.AttendanceCorrections, RecurrenceLock: module.RecurrenceLock,
		}, nil
	}
	root.Rows = func(db *bun.DB, clocks ...func() time.Time) (timetableAPI.TimetableRows, error) {
		rows, err := api.NewTimetableHTTPTestRows(testpkg.MustDatabase(db), clocks...)
		if err != nil {
			return timetableAPI.TimetableRows{}, err
		}
		rooms, err := api.NewTimetableRoomRows(testpkg.MustDatabase(db))
		if err != nil {
			return timetableAPI.TimetableRows{}, err
		}
		calendar, err := api.NewTimetableSchoolCalendar(testpkg.MustDatabase(db))
		if err != nil {
			return timetableAPI.TimetableRows{}, err
		}
		return timetableAPI.TimetableRows{
			Repositories: rows.Repositories, Timetable: rows.Timetable, ShiftType: rows.ShiftType, Room: rooms.Room,
			CalendarPeriods: calendar.SchoolCalendar(), PeriodUsage: rows.CalendarPeriodUsage(),
		}, nil
	}
	root.People = func(db *bun.DB) (timetableAPI.People, error) {
		return api.NewTimetableHTTPTestPeople(testpkg.MustDatabase(db))
	}
	root.RecurrenceLock = func(db *bun.DB) (timetableModule.RecurrenceWriteLock, error) {
		return api.NewTimetableHTTPTestRecurrenceLock(testpkg.MustDatabase(db))
	}
	root.SessionRecords = func(db *bun.DB) timetableAPI.SessionRecords {
		return api.NewTimetableHTTPTestSessionRecords(testpkg.MustDatabase(db))
	}
	root.OfferingSources = func(t *testing.T, db *bun.DB) (timetableModule.OfferingSourceSupport, error) {
		t.Helper()
		return api.NewTimetableHTTPTestOfferingSources(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	}
	root.StaffAccountPeople = func(personID, staffID int64) timetablePorts.OperationPeople {
		return api.TimetableTestStaffAccountPeople{PersonID: personID, StaffID: staffID}
	}
	root.ErrCareOfferingInvalid = api.TimetableHTTPTestCareOfferingInvalid
}
