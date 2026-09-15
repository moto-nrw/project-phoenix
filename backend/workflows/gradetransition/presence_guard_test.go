package gradetransition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// presenceStub answers the check-in guard from canned rows and records the
// filters the guard asked with.
type presenceStub struct {
	visits         []studentpresence.Visit
	attendance     []studentpresence.Attendance
	visitErr       error
	attendanceErr  error
	visitFilter    *studentpresence.VisitFilter
	attendanceFilt *studentpresence.AttendanceFilter
}

func (p *presenceStub) ListVisits(_ context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	p.visitFilter = &filter
	return p.visits, p.visitErr
}

func (p *presenceStub) ListAttendance(_ context.Context, filter studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	p.attendanceFilt = &filter
	return p.attendance, p.attendanceErr
}

func guardWorkflow(presence Presence) *Workflow {
	return &Workflow{deps: Dependencies{Presence: presence, Today: func() string { return "2026-08-24" }}}
}

// The guard refuses a graduation while any graduating child is checked in:
// an open visit, or today's newest attendance record without a checkout.
func TestGraduationGuardRefusesCheckedInChildren(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	graduates := []cohortStudent{{StudentID: 11, SchoolClass: "4a"}, {StudentID: 12, SchoolClass: "4a"}}
	checkedOut := time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)

	t.Run("no graduates asks nothing", func(t *testing.T) {
		t.Parallel()
		stub := &presenceStub{}
		require.NoError(t, guardWorkflow(stub).ensureGraduatesNotCheckedIn(ctx, nil))
		assert.Nil(t, stub.visitFilter)
		assert.Nil(t, stub.attendanceFilt)
	})

	t.Run("asks for open visits and today's attendance of exactly the cohort", func(t *testing.T) {
		t.Parallel()
		stub := &presenceStub{}
		require.NoError(t, guardWorkflow(stub).ensureGraduatesNotCheckedIn(ctx, graduates))
		require.NotNil(t, stub.visitFilter)
		assert.Equal(t, []int64{11, 12}, stub.visitFilter.StudentIDs)
		assert.True(t, stub.visitFilter.OpenOnly, "only an open visit means checked in")
		require.NotNil(t, stub.attendanceFilt)
		assert.Equal(t, []int64{11, 12}, stub.attendanceFilt.StudentIDs)
		assert.Equal(t, "2026-08-24", stub.attendanceFilt.FromDate)
		assert.Equal(t, "2026-08-24", stub.attendanceFilt.UntilDate)
		assert.True(t, stub.attendanceFilt.NewestFirst, "the newest record per child decides")
		assert.True(t, stub.attendanceFilt.StudentOrder)
	})

	t.Run("an open visit blocks", func(t *testing.T) {
		t.Parallel()
		stub := &presenceStub{visits: []studentpresence.Visit{{StudentID: 12}}}
		err := guardWorkflow(stub).ensureGraduatesNotCheckedIn(ctx, graduates)
		require.ErrorIs(t, err, ErrGraduatesCheckedIn)
		assert.ErrorContains(t, err, "1 student(s) must be checked out first")
	})

	t.Run("an open attendance record without a visit blocks", func(t *testing.T) {
		t.Parallel()
		stub := &presenceStub{attendance: []studentpresence.Attendance{{StudentID: 11, Date: "2026-08-24"}}}
		err := guardWorkflow(stub).ensureGraduatesNotCheckedIn(ctx, graduates)
		require.ErrorIs(t, err, ErrGraduatesCheckedIn)
	})

	t.Run("the newest attendance record wins over an older open one", func(t *testing.T) {
		t.Parallel()
		stub := &presenceStub{attendance: []studentpresence.Attendance{
			{StudentID: 11, CheckOutTime: &checkedOut},
			{StudentID: 11},
		}}
		require.NoError(t, guardWorkflow(stub).ensureGraduatesNotCheckedIn(ctx, graduates))
	})

	t.Run("two checked-in children are counted once each", func(t *testing.T) {
		t.Parallel()
		stub := &presenceStub{
			visits:     []studentpresence.Visit{{StudentID: 11}, {StudentID: 11}},
			attendance: []studentpresence.Attendance{{StudentID: 12}},
		}
		err := guardWorkflow(stub).ensureGraduatesNotCheckedIn(ctx, graduates)
		assert.ErrorContains(t, err, "2 student(s) must be checked out first")
	})

	t.Run("a presence read failure propagates instead of waving the apply through", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("presence unavailable")
		err := guardWorkflow(&presenceStub{visitErr: boom}).ensureGraduatesNotCheckedIn(ctx, graduates)
		require.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, ErrGraduatesCheckedIn)
		err = guardWorkflow(&presenceStub{attendanceErr: boom}).ensureGraduatesNotCheckedIn(ctx, graduates)
		require.ErrorIs(t, err, boom)
	})
}
