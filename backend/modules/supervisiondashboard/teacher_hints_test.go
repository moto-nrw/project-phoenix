package supervisiondashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teacherHintReader struct {
	hints []TeacherHint
	err   error
	asked [][]int64
	dates []Date
}

func (r *teacherHintReader) TeacherHints(_ context.Context, ids []int64, date Date) ([]TeacherHint, error) {
	r.asked = append(r.asked, ids)
	r.dates = append(r.dates, date)
	return r.hints, r.err
}

// TestLoadPlanningTimesAttachesTeacherHints pins that the Lehrkraft's day
// hints (#4795) share the gate of the planning times: users:read and full
// access, for the children in the projection and the business day.
func TestLoadPlanningTimesAttachesTeacherHints(t *testing.T) {
	t.Parallel()
	const date Date = "2026-08-19"
	hint := TeacherHint{StudentID: 42, Body: "Hat Bauchweh", AuthorName: "Frau Kurz"}
	noTimes := func(context.Context, []int64, Date) (map[int64]Pickup, error) { return map[int64]Pickup{}, nil }
	noArrivals := func(context.Context, []int64, Date) (map[int64]Arrival, error) { return map[int64]Arrival{}, nil }

	reader := &teacherHintReader{hints: []TeacherHint{hint}}
	svc := newService(&fakes{pickups: noTimes, arrivals: noArrivals})
	svc.deps.TeacherHints = reader
	projection := emptyProjection()
	require.NoError(t, svc.loadPlanningTimes(context.Background(), projection, Caller{CanReadStudents: true}, []int64{42, 43}, true, date))
	assert.Equal(t, []TeacherHint{hint}, projection.TeacherHints)
	assert.Equal(t, [][]int64{{42, 43}}, reader.asked)
	assert.Equal(t, []Date{date}, reader.dates)

	for name, gate := range map[string]struct {
		caller     Caller
		fullAccess bool
	}{
		"without users:read":  {Caller{}, true},
		"without full access": {Caller{CanReadStudents: true}, false},
	} {
		reader := &teacherHintReader{hints: []TeacherHint{hint}}
		svc := newService(&fakes{pickups: noTimes, arrivals: noArrivals})
		svc.deps.TeacherHints = reader
		projection := emptyProjection()
		require.NoError(t, svc.loadPlanningTimes(context.Background(), projection, gate.caller, []int64{42}, gate.fullAccess, date), name)
		assert.Empty(t, projection.TeacherHints, name)
		assert.Empty(t, reader.asked, name)
	}

	failure := errors.New("directory unavailable")
	svc = newService(&fakes{pickups: noTimes, arrivals: noArrivals})
	svc.deps.TeacherHints = &teacherHintReader{err: failure}
	err := svc.loadPlanningTimes(context.Background(), emptyProjection(), Caller{CanReadStudents: true}, []int64{42}, true, date)
	require.ErrorIs(t, err, failure)
}
