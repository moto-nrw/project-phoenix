package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// vanishedExceptions answers like the storage adapter once a concurrent
// delete has removed the row.
type vanishedExceptions struct {
	ports.EffectiveExceptionRepository[*careplan.PickupException]
}

func (vanishedExceptions) FindByID(context.Context, int64) (*careplan.PickupException, error) {
	return nil, careplan.ErrStudentScheduleNotFound
}

type passThroughTransaction struct {
	ports.PickupScheduleTransaction
}

func (passThroughTransaction) WithinTenant(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (passThroughTransaction) LockStudentAndExceptionDay(context.Context, int64, string) error {
	return nil
}
func (passThroughTransaction) IsNotFound(err error) bool {
	return errors.Is(err, careplan.ErrStudentScheduleNotFound)
}

type idleAutoExcusal struct{ careplan.PickupAutoExcusal }

func TestPickupExceptionUpdateReportsNotFoundWhenRowVanishedUnderLock(t *testing.T) {
	t.Parallel()
	for name, auto := range map[string]careplan.PickupAutoExcusal{"without auto excusal": nil, "with auto excusal": idleAutoExcusal{}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service := NewPickupSchedules(nil, vanishedExceptions{}, nil, passThroughTransaction{}, nil, auto, nil, nil)

			_, err := service.UpdateException(context.Background(), 41, 42, "2026-09-21", nil, nil, true, nil)

			require.ErrorIs(t, err, careplan.ErrCareExceptionNotFound)
		})
	}
}

// racedExceptions answers like the storage adapter when a concurrent delete
// removes the row between the unlocked lookup and the locked re-read.
type racedExceptions struct {
	ports.EffectiveExceptionRepository[*careplan.PickupException]
}

func (racedExceptions) FindByID(context.Context, int64) (*careplan.PickupException, error) {
	return &careplan.PickupException{ID: 41, StudentID: 42, ExceptionDate: "2026-09-21"}, nil
}

func (racedExceptions) FindByIDForUpdate(context.Context, int64) (*careplan.PickupException, error) {
	return nil, careplan.ErrStudentScheduleNotFound
}

func TestPickupExceptionDeleteStaysIdempotentWhenRowVanishedUnderLock(t *testing.T) {
	t.Parallel()
	service := NewPickupSchedules(nil, racedExceptions{}, nil, passThroughTransaction{}, nil, nil, nil, nil)

	err := service.DeleteStudentPickupException(context.Background(), 41, 42)

	require.NoError(t, err)
}
