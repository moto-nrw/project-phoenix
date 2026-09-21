// These unit tests use in-memory port stubs and do not open a database.
package presenceservice

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type attendancePortForFacadeTest struct {
	presence.StudentPresence
	has       bool
	err       error
	gotFilter studentpresence.AttendanceFilter
}

func (r *attendancePortForFacadeTest) HasAttendance(_ context.Context, filter studentpresence.AttendanceFilter) (bool, error) {
	r.gotFilter = filter
	return r.has, r.err
}

type roomPortForFacadeTest struct {
	AttendanceRooms
	rooms  []*SessionRoom
	err    error
	gotIDs []int64
}

func (r *roomPortForFacadeTest) FindByIDs(_ context.Context, ids []int64) ([]*SessionRoom, error) {
	r.gotIDs = append([]int64(nil), ids...)
	return r.rooms, r.err
}

func TestRoomProjectionPreservesMissingAndPartialResults(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("room lookup failed")
	for _, tc := range []struct {
		name string
		rows []*SessionRoom
		err  error
		want []*studentpresence.SessionRoomSummary
	}{
		{name: "nil"},
		{name: "empty", rows: []*SessionRoom{}, want: []*studentpresence.SessionRoomSummary{}},
		{name: "partial error", rows: []*SessionRoom{nil, {ID: 7, Name: "Aula", IsOpenRoom: true}}, err: lookupErr,
			want: []*studentpresence.SessionRoomSummary{nil, {ID: 7, Name: "Aula"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facade := &presenceFacade{rooms: &roomPortForFacadeTest{rooms: tc.rows, err: tc.err}}
			got, err := facade.GetRoomsByIDs(context.Background(), []int64{7})
			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPresenceFacadePortReads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("has open attendance delegates date and result", func(t *testing.T) {
		date := timezone.DateFromTime(timezone.NewDate(2026, 8, 24).BerlinMidnight())
		port := &attendancePortForFacadeTest{has: true}
		facade := &presenceFacade{attendance: port}

		hasOpen, err := facade.HasOpenAttendanceOn(ctx, date)

		require.NoError(t, err)
		assert.True(t, hasOpen)
		assert.Equal(t, studentpresence.AttendanceFilter{FromDate: date.String(), UntilDate: date.String(), OpenOnly: true}, port.gotFilter)
	})

	t.Run("has open attendance preserves repository error", func(t *testing.T) {
		expectedErr := errors.New("attendance lookup failed")
		facade := &presenceFacade{attendance: &attendancePortForFacadeTest{err: expectedErr}}

		hasOpen, err := facade.HasOpenAttendanceOn(ctx, timezone.NewDate(2026, 8, 24))

		require.ErrorIs(t, err, expectedErr)
		assert.False(t, hasOpen)
	})

	t.Run("get rooms by ids delegates ids and result", func(t *testing.T) {
		port := &roomPortForFacadeTest{rooms: []*SessionRoom{{Name: "Aula"}}}
		facade := &presenceFacade{rooms: port}

		got, err := facade.GetRoomsByIDs(ctx, []int64{10, 20})

		require.NoError(t, err)
		assert.Equal(t, []*studentpresence.SessionRoomSummary{{Name: "Aula"}}, got)
		assert.Equal(t, []int64{10, 20}, port.gotIDs)
	})
}

// A missing timetable must answer the tenant-wide care-plan signal with false
// instead of panicking.
func TestStudentHistory_HasPlannedSlotsInRange_NilSlotRepo(t *testing.T) {
	t.Parallel()

	history := NewStudentHistory(nil, nil, nil, nil)

	has, err := history.HasPlannedSlotsInRange(context.Background(), timezone.NewDate(2032, 3, 2), timezone.NewDate(2032, 3, 6))
	require.NoError(t, err)
	assert.False(t, has, "without a slot repository there is no care-plan signal")
}
