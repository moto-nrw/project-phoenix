package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tripScope struct{ compose.SchoolWriteScope }

func (tripScope) SchoolMayReportClassTrips(context.Context) (bool, error) { return true, nil }

type tripRosters struct{}

func (tripRosters) ClassRosterDay(context.Context, string, calendar.Date) (*compose.DayRoster, error) {
	return &compose.DayRoster{Rows: []compose.DayRosterRow{{StudentID: 11}, {StudentID: 12}}}, nil
}

type tripArrivals struct {
	careplan.ArrivalScheduleService
	calls []int64
	err   error
}

func (f *tripArrivals) CreateOrReclaimExceptionForClass(_ context.Context, schoolClass string, studentID int64, _ calendar.Date, _ *time.Time, _ *string, _ int64, _ func() (int64, error)) (*careplan.ArrivalException, error) {
	if schoolClass != "3b" {
		panic("unexpected school class")
	}
	f.calls = append(f.calls, studentID)
	return &careplan.ArrivalException{}, f.err
}

type tripAnnouncer struct{ calls int }

func (a *tripAnnouncer) AnnounceArrivalScheduleChange(context.Context) { a.calls++ }

func TestClassTripReturnTimeLocksInOrderAndMapsMembershipRejection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		err           error
		wantIDs       []int64
		wantErr       error
		announcements int
	}{
		{name: "sorted and deduplicated", wantIDs: []int64{11, 12}, announcements: 1},
		{name: "reassigned after roster read", err: careplan.ErrBulkStudentNotFound, wantIDs: []int64{11}, wantErr: classday.ErrClassTripStudentNotInClass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			arrivals := &tripArrivals{err: tc.err}
			announcer := &tripAnnouncer{}
			service := compose.NewClassDay(compose.ClassDayDependencies{
				Caller: struct{ compose.CallerReader }{}, Rosters: tripRosters{}, WriteScope: tripScope{},
				ClassArrivalExceptions: arrivals, TripStatusDays: struct{ compose.ClassTripStatusDays }{}, Announcer: announcer,
			})
			returnTime := time.Date(0, 1, 1, 13, 30, 0, 0, time.UTC)
			_, err := service.ReportClassTrip(context.Background(), classday.ClassTripWrite{
				SchoolClass: "3b", From: "2099-03-09", To: "2099-03-09", StudentIDs: []int64{12, 11, 12}, ReturnTime: &returnTime,
			})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.wantIDs, arrivals.calls)
			assert.Equal(t, tc.announcements, announcer.calls)
		})
	}
}
