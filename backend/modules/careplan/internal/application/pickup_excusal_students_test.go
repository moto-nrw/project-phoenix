package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// recordingExcusalLocker records the tenant unit of work and the student
// locks the weekly resync takes.
type recordingExcusalLocker struct {
	ports.PickupExcusalLocker
	units  int
	inUnit bool
	locked []int64
}

func (l *recordingExcusalLocker) WithinTenant(ctx context.Context, fn func(context.Context) error) error {
	l.units++
	l.inUnit = true
	defer func() { l.inUnit = false }()
	return fn(ctx)
}

func (l *recordingExcusalLocker) LockStudent(_ context.Context, id int64) error {
	if !l.inUnit {
		return errors.New("student lock taken outside the unit of work")
	}
	l.locked = append(l.locked, id)
	return nil
}

// noUpcomingExceptions answers every student without a future exception.
type noUpcomingExceptions struct{ ports.PickupExcusalStore }

func (noUpcomingExceptions) FindUpcomingByStudentID(context.Context, int64) ([]*careplan.PickupException, error) {
	return nil, nil
}

func TestResyncFutureExceptionsRunsEveryStudentInOneUnitOfWork(t *testing.T) {
	t.Parallel()
	locker := &recordingExcusalLocker{}
	syncer := NewPickupAutoExcusal(noUpcomingExceptions{}, nil, nil, nil, nil, locker)

	require.NoError(t, syncer.ResyncFutureExceptions(context.Background(), []int64{7, 3}))

	require.Equal(t, 1, locker.units)
	require.Equal(t, []int64{7, 3}, locker.locked)
}

// scriptedStudentLocks answers the People Directory lock per student.
type scriptedStudentLocks struct {
	missing map[int64]bool
	failing map[int64]error
	locked  []int64
}

func (l *scriptedStudentLocks) LockStudent(_ context.Context, id int64) (bool, error) {
	if err := l.failing[id]; err != nil {
		return false, err
	}
	if l.missing[id] {
		return false, nil
	}
	l.locked = append(l.locked, id)
	return true, nil
}

func TestLockWeeklyPickupStudentsSkipsStudentsThatNoLongerExist(t *testing.T) {
	t.Parallel()
	students := &scriptedStudentLocks{missing: map[int64]bool{2: true}}

	require.NoError(t, LockWeeklyPickupStudents(context.Background(), students, []int64{1, 2, 3}))

	require.Equal(t, []int64{1, 3}, students.locked)
}

func TestLockWeeklyPickupStudentsStopsAtTheFirstLockFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("lock timeout")
	students := &scriptedStudentLocks{failing: map[int64]error{2: failure}}

	err := LockWeeklyPickupStudents(context.Background(), students, []int64{1, 2, 3})

	require.ErrorIs(t, err, failure)
	require.Equal(t, []int64{1}, students.locked)
}

func TestLockWeeklyPickupStudentsRequiresTheStudentLocks(t *testing.T) {
	t.Parallel()
	require.Error(t, LockWeeklyPickupStudents(context.Background(), nil, []int64{1}))
}
