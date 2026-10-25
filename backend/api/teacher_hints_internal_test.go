package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type fakeSchoolDayHints struct {
	rows  []peopledirectory.SchoolDayHint
	calls int
	day   calendar.Date
}

func (f *fakeSchoolDayHints) ListSchoolDayHints(_ context.Context, day calendar.Date, _ []int64) ([]peopledirectory.SchoolDayHint, error) {
	f.calls++
	f.day = day
	return f.rows, nil
}

// TestTeacherHintBindings pins that each OGS view receives People Directory's
// day hints (#4795) in its own shape, oldest first per child, and that no
// child means no read.
func TestTeacherHintBindings(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2099, 3, 2)
	directory := func() *fakeSchoolDayHints {
		return &fakeSchoolDayHints{rows: []peopledirectory.SchoolDayHint{
			{StudentID: 7, Body: "Heute Sportfest", AuthorName: "Lena Lehrerin", SchoolClass: "3b"},
			{StudentID: 8, Body: "Hat Bauchweh", AuthorName: "Lena Lehrerin"},
			{StudentID: 7, Body: "Sportzeug fehlt", AuthorName: "Tom Lehrer"},
		}}
	}

	t.Run("group live by child", func(t *testing.T) {
		t.Parallel()
		hints := directory()
		got, err := groupLiveTeacherHintReader{hints: hints}.TeacherHints(context.Background(), []int64{7, 8}, grouplive.Date(day))
		require.NoError(t, err)
		assert.Equal(t, day, hints.day)
		assert.Equal(t, []grouplive.TeacherHint{
			{Body: "Heute Sportfest", AuthorName: "Lena Lehrerin", SchoolClass: "3b"},
			{Body: "Sportzeug fehlt", AuthorName: "Tom Lehrer"},
		}, got[7])
		assert.Equal(t, []grouplive.TeacherHint{{Body: "Hat Bauchweh", AuthorName: "Lena Lehrerin"}}, got[8])
	})

	t.Run("supervision as a list with the child", func(t *testing.T) {
		t.Parallel()
		got, err := supervisionTeacherHints{hints: directory()}.TeacherHints(context.Background(), []int64{7, 8}, supervisiondashboard.Date(day))
		require.NoError(t, err)
		assert.Equal(t, []supervisiondashboard.TeacherHint{
			{StudentID: 7, Body: "Heute Sportfest", AuthorName: "Lena Lehrerin", SchoolClass: "3b"},
			{StudentID: 8, Body: "Hat Bauchweh", AuthorName: "Lena Lehrerin"},
			{StudentID: 7, Body: "Sportzeug fehlt", AuthorName: "Tom Lehrer"},
		}, got)
	})

	t.Run("block roster by child", func(t *testing.T) {
		t.Parallel()
		got, err := newTimetableTeacherHints(directory()).TeacherHints(context.Background(), []int64{7, 8}, day)
		require.NoError(t, err)
		assert.Equal(t, []timetable.OperationTeacherHint{
			{Body: "Heute Sportfest", AuthorName: "Lena Lehrerin", SchoolClass: "3b"},
			{Body: "Sportzeug fehlt", AuthorName: "Tom Lehrer"},
		}, got[7])
	})

	t.Run("no child, no read", func(t *testing.T) {
		t.Parallel()
		hints := directory()
		got, err := groupLiveTeacherHintReader{hints: hints}.TeacherHints(context.Background(), nil, grouplive.Date(day))
		require.NoError(t, err)
		assert.Empty(t, got)
		assert.Zero(t, hints.calls)
	})

	t.Run("a missing directory is a wiring error", func(t *testing.T) {
		t.Parallel()
		_, err := groupLiveTeacherHintReader{}.TeacherHints(context.Background(), []int64{7}, grouplive.Date(day))
		require.Error(t, err)
		assert.Nil(t, newTimetableTeacherHints(nil), "the roster shows no hint without the directory")
	})
}
