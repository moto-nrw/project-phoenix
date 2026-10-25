package grouplive

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// teacherHintReader answers the Lehrkraft's day hints (#4795) from a map and
// records which children it was asked for.
type teacherHintReader struct {
	hints map[int64][]TeacherHint
	err   error
	asked [][]int64
	dates []Date
}

func (r *teacherHintReader) TeacherHints(_ context.Context, ids []int64, date Date) (map[int64][]TeacherHint, error) {
	r.asked = append(r.asked, ids)
	r.dates = append(r.dates, date)
	return r.hints, r.err
}

func teacherHintFakes(caller func(context.Context) (Caller, error), restricted bool) *fakes {
	return &fakes{
		caller:           caller,
		restricted:       restricted,
		supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
		members: func(context.Context, int64) ([]RosterStudent, error) {
			return []RosterStudent{{ID: 11, FirstName: "Ada"}, {ID: 12, FirstName: "Bob"}}, nil
		},
	}
}

func teacherHintService(f *fakes, reader TeacherHintReader) Query {
	return New(Dependencies{
		Access: f, Groups: f, Roster: f, Presence: f, Planning: f, Transfers: f,
		Settings: f, Calendar: f, TeacherHints: reader,
	})
}

// TestLiveGroupAttachesTeacherHints pins that today's hints of the Lehrkraft
// reach the child cards of the group, and only with planning access
// (staff eligibility plus users:read), like every other note of a child.
func TestLiveGroupAttachesTeacherHints(t *testing.T) {
	t.Parallel()
	hint := TeacherHint{Body: "Heute Sportfest", AuthorName: "Frau Kurz", SchoolClass: "2a"}

	reader := &teacherHintReader{hints: map[int64][]TeacherHint{11: {hint}}}
	got, err := teacherHintService(teacherHintFakes(fullCaller, false), reader).LiveGroup(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, got.Students, 2)
	assert.Equal(t, []TeacherHint{hint}, got.Students[0].TeacherHints)
	assert.Empty(t, got.Students[1].TeacherHints)
	assert.Equal(t, [][]int64{{11, 12}}, reader.asked)
	assert.Equal(t, []Date{today}, reader.dates, "the hints are today's")

	for name, f := range map[string]*fakes{
		"without users:read":        teacherHintFakes(func(context.Context) (Caller, error) { return Caller{CanReadGroups: true}, nil }, false),
		"without staff eligibility": teacherHintFakes(fullCaller, true),
	} {
		reader := &teacherHintReader{hints: map[int64][]TeacherHint{11: {hint}}}
		got, err := teacherHintService(f, reader).LiveGroup(context.Background(), 1)
		require.NoError(t, err, name)
		for _, student := range got.Students {
			assert.Empty(t, student.TeacherHints, name)
		}
		assert.Empty(t, reader.asked, "%s: no child is asked for", name)
	}

	failure := errors.New("directory unavailable")
	_, err = teacherHintService(teacherHintFakes(fullCaller, false), &teacherHintReader{err: failure}).LiveGroup(context.Background(), 1)
	require.ErrorIs(t, err, failure, "a failed read fails the projection instead of hiding the hints")
}
