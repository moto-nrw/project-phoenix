package contracttest_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

type classArrivalBatchFixture struct {
	db      *bun.DB
	service careplan.ArrivalScheduleService
	staffID int64
	date    calendar.Date
	clock   time.Time
}

func newClassArrivalBatchFixture(t *testing.T) *classArrivalBatchFixture {
	t.Helper()
	return newClassArrivalBatchFixtureOn(t, testpkg.SetupTestDB(t))
}

func newClassArrivalBatchFixtureOn(t *testing.T, db *bun.DB) *classArrivalBatchFixture {
	t.Helper()
	owner := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
	_, deps, err := owners.ArrivalScheduleBinding(people, nil, nil, nil)
	require.NoError(t, err)
	service, err := compose.NewArrivalSchedules(db, owner, nil, nil, deps)
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Batch", "Author")
	return &classArrivalBatchFixture{db: db, service: service, staffID: staff.ID,
		date: calendar.NewDate(2099, 3, 9), clock: time.Date(1, 1, 1, 13, 30, 0, 0, time.UTC)}
}

// report runs the class command for the children inside one tenant
// transaction and returns, before that transaction rolls back, which of the
// children hold an arrival exception of the day.
func (f *classArrivalBatchFixture) report(t *testing.T, ctx context.Context, class string, studentIDs []int64) (map[int64]bool, error) {
	t.Helper()
	written := map[int64]bool{}
	var reportErr error
	_ = testpkg.TenantCommand0(t, ctx, f.db, func(txCtx context.Context) error {
		reportErr = f.service.CreateOrReclaimExceptionsForClass(txCtx, class, studentIDs, f.date, &f.clock, nil, f.staffID, func() (int64, error) { return f.staffID, nil })
		for _, id := range studentIDs {
			row, err := f.service.GetStudentArrivalExceptionForDate(txCtx, id, f.date)
			require.NoError(t, err)
			written[id] = row != nil
		}
		return errRollback
	})
	return written, reportErr
}

var errRollback = errors.New("roll the report back")

// staffArrival stores an exception the OGS entered, which a class report
// may not reclaim.
func (f *classArrivalBatchFixture) staffArrival(t *testing.T, ctx context.Context, studentID int64) {
	t.Helper()
	clock := time.Date(1, 1, 1, 9, 0, 0, 0, time.UTC)
	require.NoError(t, testpkg.TenantCommand0(t, ctx, f.db, func(txCtx context.Context) error {
		return f.service.CreateStudentArrivalException(txCtx, &careplan.ArrivalException{
			StudentID: studentID, ExceptionDate: careplan.Date(f.date), ExpectedArrival: &clock, CreatedBy: f.staffID, Source: careplan.ExceptionSourceStaff,
		})
	}))
}

// A refusal stops the report where single commands would have stopped it:
// the children before it are written in order, none after it.
func TestClassArrivalBatchStopsAtTheFirstRefusal(t *testing.T) {
	t.Parallel()
	f := newClassArrivalBatchFixture(t)
	ctx := testpkg.Ctx(t)
	first := testpkg.CreateTestStudent(t, f.db, "Batch", "First", "1a")
	refused := testpkg.CreateTestStudent(t, f.db, "Batch", "Refused", "1a")
	last := testpkg.CreateTestStudent(t, f.db, "Batch", "Last", "1a")
	f.staffArrival(t, ctx, refused.ID)

	written, err := f.report(t, ctx, "1a", []int64{first.ID, refused.ID, last.ID})
	require.ErrorIs(t, err, careplan.ErrCareExceptionDayConflict)
	assert.True(t, written[first.ID], "the child before the refusal is written")
	assert.True(t, written[refused.ID], "the OGS entry stays")
	assert.False(t, written[last.ID], "no child after the refusal is written")
}

// A membership refusal of a later child is reported after the earlier
// children were checked: an earlier day conflict wins, and without one the
// earlier children are written first.
func TestClassArrivalBatchReportsTheEarliestRefusal(t *testing.T) {
	t.Parallel()
	f := newClassArrivalBatchFixture(t)
	ctx := testpkg.Ctx(t)
	conflict := testpkg.CreateTestStudent(t, f.db, "Batch", "Conflict", "1a")
	free := testpkg.CreateTestStudent(t, f.db, "Batch", "Free", "1a")
	foreign := testpkg.CreateTestStudent(t, f.db, "Batch", "Foreign", "2b")
	f.staffArrival(t, ctx, conflict.ID)

	_, err := f.report(t, ctx, "1a", []int64{conflict.ID, foreign.ID})
	require.ErrorIs(t, err, careplan.ErrCareExceptionDayConflict, "the earlier child's conflict comes first")

	written, err := f.report(t, ctx, "1a", []int64{free.ID, foreign.ID})
	require.ErrorIs(t, err, careplan.ErrBulkStudentNotFound)
	assert.True(t, written[free.ID], "the child before the refused one is written first")
	assert.False(t, written[foreign.ID])
}

// Every child is handled completely before the next one is locked, as one
// command per child: student row, care day, locked recheck, the day's
// exception read and the write (#5824). Only the single command's second
// read before the write is gone.
func TestClassArrivalBatchHandlesEachChildBeforeLockingTheNext(t *testing.T) {
	t.Parallel()
	f := newClassArrivalBatchFixture(t)
	ctx := testpkg.Ctx(t)
	ids := []int64{}
	for _, name := range []string{"A", "B", "C"} {
		ids = append(ids, testpkg.CreateTestStudent(t, f.db, "Batch", "Lock"+name, "1a").ID)
	}
	counter := testpkg.CaptureQueriesForContext(t, f.db)
	written, err := f.report(t, counter.Context(ctx), "1a", ids)
	require.NoError(t, err)
	for _, id := range ids {
		assert.True(t, written[id])
	}

	var steps []string
	for _, sql := range counter.Queries() {
		lower := strings.ToLower(sql)
		switch {
		case strings.Contains(lower, "care-exception-day"):
			steps = append(steps, "day")
		case strings.Contains(lower, "class_write_gate"):
			steps = append(steps, "recheck")
		case strings.HasPrefix(lower, `select "student".id`) && strings.Contains(lower, "for update"):
			steps = append(steps, "student")
		case strings.HasPrefix(lower, "select") && strings.Contains(lower, "student_arrival_exception"):
			steps = append(steps, "read")
		case strings.HasPrefix(lower, "insert into schedule.student_arrival_exceptions"):
			steps = append(steps, "insert")
		}
	}
	perChild := []string{"student", "day", "recheck", "read", "insert"}
	want := []string{}
	for range ids {
		want = append(want, perChild...)
	}
	// report's own check reads each child's day once more afterwards.
	for range ids {
		want = append(want, "read")
	}
	assert.Equal(t, want, steps)
}
