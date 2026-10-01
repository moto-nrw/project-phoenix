package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

type fakeWriteScope struct {
	may bool
	err error
}

func (f fakeWriteScope) SchoolMayWriteClassArrivalExceptions(context.Context) (bool, error) {
	return f.may, f.err
}

type fakeBlockStarts struct{ start string }

func (f fakeBlockStarts) EarliestPlannedBlockStartForClass(context.Context, string, timezone.Date) (string, error) {
	return f.start, nil
}

type countingAnnouncer struct{ calls int }

func (a *countingAnnouncer) AnnounceArrivalScheduleChange(context.Context) { a.calls++ }

func TestArrivalExceptionsWithoutStoreAreNotConfigured(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	var seam arrivalExceptions

	may, err := seam.MayWriteArrivalExceptions(ctx)
	assert.False(t, may)
	require.ErrorIs(t, err, classday.ErrArrivalExceptionsNotConfigured)
	_, err = seam.ArrivalExceptions(ctx, "1a", "2026-08-03", "2026-08-07")
	require.ErrorIs(t, err, classday.ErrArrivalExceptionsNotConfigured)
	_, err = seam.SetArrivalException(ctx, classday.ArrivalExceptionWrite{SchoolClass: "1a", Date: "2026-08-03"})
	require.ErrorIs(t, err, classday.ErrArrivalExceptionsNotConfigured)
	require.ErrorIs(t, seam.ClearArrivalException(ctx, "1a", "2026-08-03"), classday.ErrArrivalExceptionsNotConfigured)
	_, err = seam.EarliestBlockStart(ctx, "1a", "2026-08-03")
	require.ErrorIs(t, err, classday.ErrArrivalExceptionsNotConfigured)
}

func TestMayWriteArrivalExceptionsAppliesWriteScope(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := &fakeClassArrivalExceptions{}

	_, err := arrivalExceptions{schedule: store}.MayWriteArrivalExceptions(ctx)
	assert.EqualError(t, err, "class day arrival exceptions: settings not configured")

	may, err := arrivalExceptions{schedule: store, writeScope: fakeWriteScope{may: true}}.MayWriteArrivalExceptions(ctx)
	require.NoError(t, err)
	assert.True(t, may)

	_, err = arrivalExceptions{schedule: store, writeScope: fakeWriteScope{err: errors.New("boom")}}.MayWriteArrivalExceptions(ctx)
	assert.EqualError(t, err, "class day arrival exceptions: resolve school portal write scope: boom")
}

func TestArrivalExceptionsMapsRows(t *testing.T) {
	t.Parallel()

	reason := "Konferenz"
	created := time.Date(2026, 8, 3, 7, 0, 0, 0, time.UTC)
	store := &fakeClassArrivalExceptions{rows: []ClassArrivalException{
		{SchoolClass: "1a", Date: timezone.NewDate(2026, 8, 4), ArrivalTime: time.Date(2000, 1, 1, 9, 45, 0, 0, time.UTC), Reason: &reason, CreatedAt: created},
		{SchoolClass: "1a", Date: timezone.NewDate(2026, 8, 5), ArrivalTime: time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC), CreatedAt: created, Origin: " school "},
	}}
	seam := arrivalExceptions{schedule: store}

	got, err := seam.ArrivalExceptions(context.Background(), "1a", "2026-08-03", "2026-08-07")

	require.NoError(t, err)
	assert.Equal(t, [2]timezone.Date{timezone.NewDate(2026, 8, 3), timezone.NewDate(2026, 8, 7)}, store.listRange)
	assert.Equal(t, []classday.ArrivalException{
		// Rows older than the origin column were entered by the OGS.
		{SchoolClass: "1a", Date: "2026-08-04", ArrivalTime: "09:45", Reason: &reason, CreatedAt: "2026-08-03T07:00:00Z", Origin: classday.ArrivalExceptionOriginOGS},
		{SchoolClass: "1a", Date: "2026-08-05", ArrivalTime: "10:00", CreatedAt: "2026-08-03T07:00:00Z", Origin: classday.ArrivalExceptionOriginSchool},
	}, got)

	_, err = seam.ArrivalExceptions(context.Background(), "1a", "03.08.2026", "2026-08-07")
	assert.EqualError(t, err, `invalid date "03.08.2026": expected YYYY-MM-DD`)
}

func TestSetArrivalExceptionStampsSchoolOriginAndAnnounces(t *testing.T) {
	t.Parallel()

	store := &fakeClassArrivalExceptions{}
	announcer := &countingAnnouncer{}
	seam := arrivalExceptions{schedule: store, announcer: announcer}
	arrival := time.Date(2000, 1, 1, 9, 45, 0, 0, time.UTC)

	got, err := seam.SetArrivalException(context.Background(), classday.ArrivalExceptionWrite{
		SchoolClass: "1a", Date: "2026-08-05", ArrivalTime: arrival, CreatedBy: 9,
	})

	require.NoError(t, err)
	require.Len(t, store.upserts, 1)
	assert.Equal(t, ClassArrivalExceptionWrite{
		SchoolClass: "1a", Date: timezone.NewDate(2026, 8, 5), ArrivalTime: arrival,
		Origin: classday.ArrivalExceptionOriginSchool, CreatedBy: 9,
	}, store.upserts[0])
	assert.Equal(t, classday.ArrivalExceptionOriginSchool, got.Origin)
	assert.Equal(t, "09:45", got.ArrivalTime)
	assert.Equal(t, 1, announcer.calls)

	// A refused write announces nothing.
	store.writeErr = classday.ErrArrivalExceptionWeekend
	_, err = seam.SetArrivalException(context.Background(), classday.ArrivalExceptionWrite{SchoolClass: "1a", Date: "2026-08-08"})
	require.ErrorIs(t, err, classday.ErrArrivalExceptionWeekend)
	assert.Equal(t, 1, announcer.calls)
}

func TestClearArrivalExceptionAnnouncesOnlyAfterDelete(t *testing.T) {
	t.Parallel()

	store := &fakeClassArrivalExceptions{}
	announcer := &countingAnnouncer{}
	seam := arrivalExceptions{schedule: store, announcer: announcer}

	require.NoError(t, seam.ClearArrivalException(context.Background(), "1a", "2026-08-05"))
	assert.Equal(t, []string{"1a@2026-08-05"}, store.deletes)
	assert.Equal(t, 1, announcer.calls)

	store.writeErr = classday.ErrArrivalExceptionNotFound
	require.ErrorIs(t, seam.ClearArrivalException(context.Background(), "1a", "2026-08-06"), classday.ErrArrivalExceptionNotFound)
	assert.Equal(t, 1, announcer.calls)
}

func TestEarliestBlockStartRequiresBlockStarts(t *testing.T) {
	t.Parallel()

	store := &fakeClassArrivalExceptions{}

	_, err := arrivalExceptions{schedule: store}.EarliestBlockStart(context.Background(), "1a", "2026-08-05")
	assert.EqualError(t, err, "class day arrival exceptions: block starts not configured")

	start, err := arrivalExceptions{schedule: store, blockStarts: fakeBlockStarts{start: "10:15"}}.EarliestBlockStart(context.Background(), "1a", "2026-08-05")
	require.NoError(t, err)
	assert.Equal(t, "10:15", start)
}
