package compose_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// TestDayReportFromEnrollmentKeepsTheWireShape is the output golden of the
// cutover (#2701): the school portal serves the projection's DayReport
// where it served the enrollment report before, so both must marshal to the
// same JSON, field for field, including the omitted optionals.
func TestDayReportFromEnrollmentKeepsTheWireShape(t *testing.T) {
	t.Parallel()

	reportedAt := timezone.NewDate(2026, 8, 5).BerlinMidnight().Add(7 * time.Hour)
	source := &enrollment.ClassDayReport{
		SchoolClass:     "4a",
		Date:            timezone.NewDate(2026, 8, 5),
		Weekday:         "wed",
		SchoolDay:       true,
		PhaseName:       "Schuljahr 2026/27",
		EnrollmentKnown: true,
		Totals:          enrollment.ClassDayTotals{Students: 3, Staying: 1, Leaving: 1, Absent: 1, ListEntries: 1},
		Rows: []enrollment.ClassDayRow{
			{
				StudentID: 11, FirstName: "Klara", LastName: "Klassentag", GroupName: "Delfine", Registered: true,
				StaysToday: true, Offerings: []string{"Lernzeit"}, Arrival: "11:45", Pickup: "15:00", Departure: "Abholung",
				PickupChanged: true, PickupRegular: "16:00", ReportedAt: &reportedAt,
			},
			{
				StudentID: 12, FirstName: "Nico", LastName: "Krank", Registered: true, Offerings: []string{},
				Departure: "Keine Angabe", Status: "sick", ReportedAt: &reportedAt,
			},
			{
				FirstName: "Lisa", LastName: "NurListe", ListEntry: true, ListEntryID: 9007199254740993, Offerings: []string{},
			},
		},
		ClassArrivalException: &enrollment.ClassDayArrivalException{ArrivalTime: "12:45", Reason: "Unterricht fällt aus", Origin: "school"},
	}

	want, err := json.Marshal(source)
	require.NoError(t, err)
	got, err := json.Marshal(compose.DayReportFromEnrollment(source))
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got))

	// The optionals stay omitted on both sides.
	bare, err := json.Marshal(compose.DayReportFromEnrollment(&enrollment.ClassDayReport{SchoolClass: "1b", Date: timezone.NewDate(2026, 8, 8), Rows: []enrollment.ClassDayRow{}}))
	require.NoError(t, err)
	assert.NotContains(t, string(bare), "class_arrival_exception")
	assert.NotContains(t, string(bare), "phase_name")
	assert.Nil(t, compose.DayReportFromEnrollment(nil))
}

// TestArrivalExceptionErrorsKeepTheirMessages pins the error contract of the
// class-day write seam: the projection's sentinels carry the retained
// service's messages, and a wrapped service error still classifies through
// errors.Is against the sentinel while the client reads the original text.
func TestArrivalExceptionErrorsKeepTheirMessages(t *testing.T) {
	t.Parallel()

	pairs := map[error]error{
		classday.ErrArrivalExceptionPastDate:      enrollment.ErrClassDayArrivalExceptionPastDate,
		classday.ErrArrivalExceptionWeekend:       enrollment.ErrClassDayArrivalExceptionWeekend,
		classday.ErrArrivalExceptionClassNotFound: enrollment.ErrClassDayArrivalExceptionClassNotFound,
		classday.ErrArrivalExceptionNotFound:      enrollment.ErrClassDayArrivalExceptionNotFound,
	}
	for sentinel, legacy := range pairs {
		assert.Equal(t, legacy.Error(), sentinel.Error())
	}

	wrapped := &scheduleSvc.ScheduleError{Op: "upsert class arrival exception", Err: scheduleSvc.ErrClassArrivalExceptionPastDate}
	service := compose.NewClassDay(compose.ClassDayDependencies{
		Reports:           stubReports{},
		Caller:            stubCaller{},
		ArrivalExceptions: stubArrivalExceptions{err: wrapped},
	})
	err := service.ClearArrivalException(t.Context(), "4a", classday.Date("2099-03-02"))
	require.Error(t, err)
	assert.ErrorIs(t, err, classday.ErrArrivalExceptionPastDate)
	assert.NotErrorIs(t, err, classday.ErrArrivalExceptionWeekend)
	assert.Equal(t, wrapped.Error(), err.Error(), "the client keeps reading the retained service's text")

	// A malformed calendar day is refused before the seam is reached.
	err = service.ClearArrivalException(t.Context(), "4a", classday.Date("02.03.2099"))
	require.Error(t, err)
	assert.NotErrorIs(t, err, classday.ErrArrivalExceptionPastDate)
}

type stubReports struct{}

func (stubReports) ClassDay(context.Context, string, timezone.Date, int64, string) (*enrollment.ClassDayReport, error) {
	return &enrollment.ClassDayReport{}, nil
}

type stubCaller struct{}

func (stubCaller) GetMySchoolClasses(context.Context) ([]string, error) { return []string{"4a"}, nil }
func (stubCaller) GetCurrentStaff(context.Context) (*userModels.Staff, error) {
	return &userModels.Staff{}, nil
}

type stubArrivalExceptions struct{ err error }

func (s stubArrivalExceptions) SchoolMayWrite(context.Context) (bool, error) { return true, nil }
func (s stubArrivalExceptions) List(context.Context, string, timezone.Date, timezone.Date) ([]enrollment.ClassDayArrivalExceptionEntry, error) {
	return nil, s.err
}
func (s stubArrivalExceptions) Set(context.Context, enrollment.ClassDayArrivalExceptionWrite) (*enrollment.ClassDayArrivalExceptionEntry, error) {
	return nil, s.err
}
func (s stubArrivalExceptions) Remove(context.Context, string, timezone.Date) error { return s.err }
func (s stubArrivalExceptions) EarliestBlockStart(context.Context, string, timezone.Date) (string, error) {
	return "", s.err
}
