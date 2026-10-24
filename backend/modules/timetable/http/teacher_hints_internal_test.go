package timetablehttp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type rosterTeacherHints struct {
	hints map[int64][]timetable.OperationTeacherHint
	err   error
	asked [][]int64
	days  []calendar.Date
}

func (f *rosterTeacherHints) TeacherHints(_ context.Context, ids []int64, day calendar.Date) (map[int64][]timetable.OperationTeacherHint, error) {
	f.asked = append(f.asked, ids)
	f.days = append(f.days, day)
	return f.hints, f.err
}

// TestAttachTeacherHints pins the block roster side of #4795: the Lehrkraft's
// hints of the block's day reach the roster rows, behind users:read only.
func TestAttachTeacherHints(t *testing.T) {
	t.Parallel()
	hint := timetable.OperationTeacherHint{Body: "Hat Bauchweh", AuthorName: "Lena Lehrerin"}
	roster := func() *timetable.OperationRoster {
		return &timetable.OperationRoster{
			Instance: timetable.OperationRosterInstance{Date: "2099-03-02"},
			Rows:     []timetable.OperationRosterRow{{StudentID: 7}, {StudentID: 8}},
		}
	}
	reading := testpkg.WithPermissionsContext(context.Background(), []string{"users:read"})

	hints := &rosterTeacherHints{hints: map[int64][]timetable.OperationTeacherHint{7: {hint}}}
	got := roster()
	require.NoError(t, (&Resource{Dependencies{TeacherHints: hints}}).attachTeacherHints(reading, got))
	assert.Equal(t, []timetable.OperationTeacherHint{hint}, got.Rows[0].TeacherHints)
	assert.Empty(t, got.Rows[1].TeacherHints)
	assert.Equal(t, [][]int64{{7, 8}}, hints.asked)
	assert.Equal(t, []calendar.Date{calendar.NewDate(2099, 3, 2)}, hints.days, "the block's day, not today")

	// The school portal's own supervisions carry no users:read and show none.
	hints = &rosterTeacherHints{hints: map[int64][]timetable.OperationTeacherHint{7: {hint}}}
	got = roster()
	school := testpkg.WithPermissionsContext(context.Background(), []string{"supervision:own"})
	require.NoError(t, (&Resource{Dependencies{TeacherHints: hints}}).attachTeacherHints(school, got))
	assert.Empty(t, got.Rows[0].TeacherHints)
	assert.Empty(t, hints.asked)

	failure := errors.New("directory unavailable")
	err := (&Resource{Dependencies{TeacherHints: &rosterTeacherHints{err: failure}}}).attachTeacherHints(reading, roster())
	require.ErrorIs(t, err, failure)

	require.NoError(t, (&Resource{}).attachTeacherHints(reading, roster()), "without the port no hint is attached")
}
