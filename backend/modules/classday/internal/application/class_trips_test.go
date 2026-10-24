package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type fakeTripScope struct {
	may bool
	err error
}

func (f fakeTripScope) SchoolMayReportClassTrips(context.Context) (bool, error) { return f.may, f.err }

type recordedTrip struct {
	schoolClass string
	studentIDs  []int64
	dates       []calendar.Date
	reason      string
}

type fakeTripStatusDays struct {
	calls []recordedTrip
	err   error
}

func (f *fakeTripStatusDays) RecordClassTrip(_ context.Context, schoolClass string, studentIDs []int64, dates []calendar.Date, reason string) error {
	f.calls = append(f.calls, recordedTrip{schoolClass: schoolClass, studentIDs: studentIDs, dates: dates, reason: reason})
	return f.err
}

// fakeStudentArrivals stands in for Care Plan's arrival write; a call
// panics, so every test here proves the status-day path or a refusal never
// sets an arrival. The return-time path runs against Care Plan in the HTTP
// integration tests.
type fakeStudentArrivals struct{ studentArrivals }

// tripToday is a Monday.
var tripToday = calendar.NewDate(2026, 11, 2)

func newTripSeam(statusDays *fakeTripStatusDays, arrivals *fakeStudentArrivals, announcer *countingAnnouncer) classTrips {
	seam := classTrips{
		rosters: fakeDayRosters{roster: &DayRoster{Rows: []DayRosterRow{
			{StudentID: 11}, {StudentID: 12}, {ListEntry: true, ListEntryID: 99},
		}}},
		writeScope: fakeTripScope{may: true},
		statusDays: statusDays,
		arrivals:   arrivals,
		today:      func() calendar.Date { return tripToday },
	}
	if announcer != nil {
		seam.announcer = announcer
	}
	return seam
}

func TestClassTripsWithoutWritesAreNotConfigured(t *testing.T) {
	t.Parallel()

	var seam classTrips
	may, err := seam.MayReportClassTrips(context.Background())
	assert.False(t, may)
	require.ErrorIs(t, err, classday.ErrClassTripsNotConfigured)
	_, err = seam.ReportClassTrip(context.Background(), classday.ClassTripWrite{})
	require.ErrorIs(t, err, classday.ErrClassTripsNotConfigured)
}

func TestMayReportClassTripsAppliesWriteScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	seam := newTripSeam(&fakeTripStatusDays{}, &fakeStudentArrivals{}, nil)

	may, err := seam.MayReportClassTrips(ctx)
	require.NoError(t, err)
	assert.True(t, may)

	seam.writeScope = nil
	_, err = seam.MayReportClassTrips(ctx)
	assert.EqualError(t, err, "class day class trips: settings not configured")

	seam.writeScope = fakeTripScope{err: errors.New("boom")}
	_, err = seam.MayReportClassTrips(ctx)
	assert.EqualError(t, err, "class day class trips: resolve school portal write scope: boom")
}

// A trip without a return time is the status "Klassenfahrt" for every day,
// written once for all children through the OGS's status-day write.
func TestReportClassTripWritesStatusDaysForEveryDay(t *testing.T) {
	t.Parallel()

	statusDays := &fakeTripStatusDays{}
	arrivals := &fakeStudentArrivals{}
	reason := "  Jugendherberge  "
	result, err := newTripSeam(statusDays, arrivals, nil).ReportClassTrip(context.Background(), classday.ClassTripWrite{
		SchoolClass: "3b", From: "2026-11-04", To: "2026-11-06", StudentIDs: []int64{12, 11, 12}, Reason: &reason, CreatedBy: 7,
	})
	require.NoError(t, err)

	assert.Equal(t, &classday.ClassTripResult{Kind: classday.ClassTripKindStatusDays, StudentCount: 2, DateCount: 3}, result)
	require.Len(t, statusDays.calls, 1)
	assert.Equal(t, recordedTrip{
		schoolClass: "3b", studentIDs: []int64{12, 11}, reason: "Jugendherberge",
		dates: []calendar.Date{calendar.NewDate(2026, 11, 4), calendar.NewDate(2026, 11, 5), calendar.NewDate(2026, 11, 6)},
	}, statusDays.calls[0])
}

func TestReportClassTripRefusesInvalidReports(t *testing.T) {
	t.Parallel()

	returnTime := time.Date(0, 1, 1, 13, 30, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   classday.ClassTripWrite
		want error
	}{
		{name: "no children", in: classday.ClassTripWrite{From: "2026-11-02", To: "2026-11-02"}, want: classday.ErrClassTripNoStudents},
		{name: "past start", in: classday.ClassTripWrite{From: "2026-10-30", To: "2026-11-02", StudentIDs: []int64{11}}, want: classday.ErrClassTripPastDate},
		{name: "end before start", in: classday.ClassTripWrite{From: "2026-11-04", To: "2026-11-03", StudentIDs: []int64{11}}, want: classday.ErrClassTripInvalidRange},
		{name: "longer than 31 days", in: classday.ClassTripWrite{From: "2026-11-02", To: "2026-12-03", StudentIDs: []int64{11}}, want: classday.ErrClassTripInvalidRange},
		{name: "return time on several days", in: classday.ClassTripWrite{From: "2026-11-02", To: "2026-11-03", StudentIDs: []int64{11}, ReturnTime: &returnTime}, want: classday.ErrClassTripReturnTimeMultiDay},
		{name: "return time on a Saturday", in: classday.ClassTripWrite{From: "2026-11-07", To: "2026-11-07", StudentIDs: []int64{11}, ReturnTime: &returnTime}, want: classday.ErrClassTripReturnTimeWeekend},
		{name: "child of another class", in: classday.ClassTripWrite{From: "2026-11-02", To: "2026-11-02", StudentIDs: []int64{11, 42}}, want: classday.ErrClassTripStudentNotInClass},
		{name: "class-list entry without OGS record", in: classday.ClassTripWrite{From: "2026-11-02", To: "2026-11-02", StudentIDs: []int64{99}}, want: classday.ErrClassTripStudentNotInClass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			statusDays := &fakeTripStatusDays{}
			arrivals := &fakeStudentArrivals{}
			tc.in.SchoolClass = "3b"
			_, err := newTripSeam(statusDays, arrivals, nil).ReportClassTrip(context.Background(), tc.in)
			require.ErrorIs(t, err, tc.want)
			assert.Empty(t, statusDays.calls, "a refused report writes no status day")
		})
	}
}

// A single trip day on Saturday without a return time is a status day like
// the OGS's class-trip dialog writes one.
func TestReportClassTripAllowsWeekendStatusDays(t *testing.T) {
	t.Parallel()

	statusDays := &fakeTripStatusDays{}
	_, err := newTripSeam(statusDays, &fakeStudentArrivals{}, nil).ReportClassTrip(context.Background(), classday.ClassTripWrite{
		SchoolClass: "3b", From: "2026-11-07", To: "2026-11-08", StudentIDs: []int64{11},
	})
	require.NoError(t, err)
	require.Len(t, statusDays.calls, 1)
	assert.Len(t, statusDays.calls[0].dates, 2)
}

// The write re-checks the school's release itself.
func TestReportClassTripRefusesWhenClosed(t *testing.T) {
	t.Parallel()

	statusDays := &fakeTripStatusDays{}
	seam := newTripSeam(statusDays, &fakeStudentArrivals{}, nil)
	seam.writeScope = fakeTripScope{may: false}
	_, err := seam.ReportClassTrip(context.Background(), classday.ClassTripWrite{
		SchoolClass: "3b", From: "2026-11-04", To: "2026-11-04", StudentIDs: []int64{11},
	})
	require.ErrorIs(t, err, classday.ErrClassTripsDisabled)
	assert.Empty(t, statusDays.calls)
}
