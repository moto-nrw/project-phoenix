package presence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// stayRepository serves the day's stays and records the one revision.
type stayRepository struct {
	StudentPresence
	rows    []studentpresence.Attendance
	revised *studentpresence.Attendance
}

func (r *stayRepository) LockStudentAttendance(context.Context, int64) error { return nil }

func (r *stayRepository) ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	return r.rows, nil
}

func (r *stayRepository) ReviseAttendance(_ context.Context, row studentpresence.Attendance) (studentpresence.Attendance, error) {
	r.revised = &row
	return row, nil
}

func reviseStays(t *testing.T, rows []studentpresence.Attendance, previous, updated studentpresence.Visit) *studentpresence.Attendance {
	t.Helper()
	repo := &stayRepository{rows: rows}
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: repo}}
	require.NoError(t, svc.syncAttendanceForVisitRevision(context.Background(), &previous, &updated))
	return repo.revised
}

func at(clock string) time.Time {
	parsed, err := time.ParseInLocation("15:04", clock, calendar.Berlin)
	if err != nil {
		panic(err)
	}
	day := calendar.NewDate(2026, 11, 4).BerlinMidnight()
	return day.Add(time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute)
}

func ptr(t time.Time) *time.Time { return &t }

// Live test D6 (#5710): the stay began at 07:30, the room visit at 08:00, and
// the child left for class at 09:00. Reopening the visit must reopen the stay
// and drop the class reason, although the two did not begin together.
func TestSyncAttendanceForVisitRevision_ReopensStayThatBeganBeforeTheVisit(t *testing.T) {
	t.Parallel()
	class := studentpresence.CheckoutReasonClass
	stay := studentpresence.Attendance{ID: 1, StudentID: 200, CheckInTime: at("07:30"), CheckOutTime: ptr(at("09:00")), CheckOutReason: &class}
	visit := studentpresence.Visit{ID: 10, StudentID: 200, EntryTime: at("08:00"), ExitTime: ptr(at("09:00"))}
	reopened := visit
	reopened.ExitTime = nil

	revised := reviseStays(t, []studentpresence.Attendance{stay}, visit, reopened)
	require.NotNil(t, revised)
	assert.Equal(t, stay.ID, revised.ID)
	assert.Nil(t, revised.CheckOutTime)
	assert.Nil(t, revised.CheckOutReason)
	assert.Equal(t, at("07:30"), revised.CheckInTime, "the stay keeps its own check-in")
}

// The stay is the one the visit lies in, not an earlier or later one.
func TestSyncAttendanceForVisitRevision_PicksTheStayHoldingTheVisit(t *testing.T) {
	t.Parallel()
	class := studentpresence.CheckoutReasonClass
	morning := studentpresence.Attendance{ID: 1, StudentID: 200, CheckInTime: at("07:30"), CheckOutTime: ptr(at("08:00")), CheckOutReason: &class}
	afternoon := studentpresence.Attendance{ID: morning.ID + 1, StudentID: 200, CheckInTime: at("12:30"), CheckOutTime: ptr(at("14:00"))}
	visit := studentpresence.Visit{ID: 10, StudentID: 200, EntryTime: at("13:00"), ExitTime: ptr(at("14:00"))}
	later := visit
	later.ExitTime = ptr(at("14:15"))

	revised := reviseStays(t, []studentpresence.Attendance{morning, afternoon}, visit, later)
	require.NotNil(t, revised)
	assert.Equal(t, afternoon.ID, revised.ID)
	assert.Equal(t, at("14:15"), *revised.CheckOutTime)
}

// Regression from #6498: correcting the end of an earlier visit leaves an
// independent checkout of the stay alone, whether the stay began with the
// visit or before it.
func TestSyncAttendanceForVisitRevision_LeavesAnIndependentCheckout(t *testing.T) {
	t.Parallel()
	visit := studentpresence.Visit{ID: 10, StudentID: 200, EntryTime: at("08:00"), ExitTime: ptr(at("09:00"))}
	corrected := visit
	corrected.ExitTime = ptr(at("09:05"))
	for _, checkIn := range []string{"08:00", "07:30"} {
		for _, checkOut := range []*time.Time{nil, ptr(at("11:00"))} {
			stay := studentpresence.Attendance{ID: 1, StudentID: 200, CheckInTime: at(checkIn), CheckOutTime: checkOut}
			revised := reviseStays(t, []studentpresence.Attendance{stay}, visit, corrected)
			if revised != nil {
				assert.Equal(t, checkOut, revised.CheckOutTime, "check-in %s", checkIn)
			}
		}
	}
}

func TestUpdateVisitRejectsInvalidResultingStay(t *testing.T) {
	t.Parallel()
	previous := studentpresence.Visit{ID: 10, StudentID: 200, ActiveGroupID: 300, EntryTime: at("08:00"), ExitTime: ptr(at("09:00"))}
	for _, tt := range []struct {
		name           string
		checkIn, entry string
		exit, checkOut string
	}{
		{name: "checkout before retained check-in", checkIn: "07:30", entry: "07:00", exit: "07:15", checkOut: "09:00"},
		{name: "check-in after independent checkout", checkIn: "08:00", entry: "10:00", exit: "11:00", checkOut: "09:30"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &stayRepository{
				StudentPresence: &mockVisitRepository{
					findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
						copy := previous
						return &copy, nil
					},
				},
				rows: []studentpresence.Attendance{{ID: 1, StudentID: 200, CheckInTime: at(tt.checkIn), CheckOutTime: ptr(at(tt.checkOut))}},
			}
			svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: repo}}
			updated := previous
			updated.EntryTime, updated.ExitTime = at(tt.entry), ptr(at(tt.exit))

			err := svc.updateVisit(context.Background(), &updated)
			require.ErrorIs(t, err, studentpresence.ErrInvalidTimeRange)
			assert.Nil(t, repo.revised, "an invalid stay must not reach persistence")
		})
	}
}

// A visit correction that would cross another visit of the child is refused
// before any write (#5710): A 08–09, B 10–11, A's end moved to 12.
func TestUpdateVisitRefusesOverlapBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	a := &studentpresence.Visit{ID: 1, StudentID: 200, ActiveGroupID: 300, EntryTime: at("08:00"), ExitTime: ptr(at("09:00"))}
	b := studentpresence.Visit{ID: 2, StudentID: 200, ActiveGroupID: 301, EntryTime: at("10:00"), ExitTime: ptr(at("11:00"))}
	tests := []struct {
		name    string
		exit    *time.Time
		refused bool
	}{
		{name: "end after the later visit begins", exit: ptr(at("12:00")), refused: true},
		{name: "reopened while a later visit exists", exit: nil, refused: true},
		{name: "end exactly when the later visit begins", exit: ptr(at("10:00"))},
		{name: "end before the later visit", exit: ptr(at("09:30"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			revised := false
			repo := &mockVisitRepository{
				findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
					stored := *a
					return &stored, nil
				},
				updateFunc: func(context.Context, *studentpresence.Visit) error {
					revised = true
					return nil
				},
				listOverlappingVisitsFunc: func(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
					assert.Equal(t, []int64{200}, filter.StudentIDs)
					return []studentpresence.Visit{*a, b}, nil
				},
			}
			svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal, SchoolPresence: repo, GroupRepo: &mockGroupRepository{}}}
			updated := *a
			updated.ExitTime = tt.exit

			err := svc.updateVisit(context.Background(), &updated)
			if !tt.refused {
				require.NoError(t, err)
				return
			}
			var overlap *studentpresence.VisitOverlapError
			require.ErrorAs(t, err, &overlap)
			assert.True(t, errors.Is(err, studentpresence.ErrVisitOverlap))
			assert.Equal(t, b.ID, overlap.ConflictingVisit.ID)
			assert.Equal(t, studentpresence.VisitOverlapDetails{ConflictingVisitID: b.ID, ConflictingFrom: "10:00", ConflictingUntil: "11:00"}, overlap.ErrorDetails())
			assert.False(t, revised, "no write before the refusal")
		})
	}
}
