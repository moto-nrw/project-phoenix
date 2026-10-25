package presenceprojectionhttp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// teacherHintDirectory answers only the day-hint read of the People
// Directory capability.
type teacherHintDirectory struct {
	peopleModule.Capability
	hints []peopleModule.SchoolDayHint
	err   error
	asked [][]int64
	dates []calendar.Date
}

func (d *teacherHintDirectory) ListSchoolDayHints(_ context.Context, date calendar.Date, ids []int64) ([]peopleModule.SchoolDayHint, error) {
	d.asked = append(d.asked, ids)
	d.dates = append(d.dates, date)
	return d.hints, d.err
}

// TestEnrichWithTeacherHints pins the Kindersuche side of #4795: today's
// hints of the Lehrkraft reach the rows the caller fully reads, behind
// users:read, and no other planning date shows them.
func TestEnrichWithTeacherHints(t *testing.T) {
	t.Parallel()
	// The plan carries the day and whether it is today; no live clock is read.
	today := calendar.NewDate(2099, time.March, 2)
	reading := testpkg.WithPermissionsContext(context.Background(), []string{"users:read"})
	page := func() []StudentResponse {
		return []StudentResponse{{ID: 1, HasFullAccess: true}, {ID: 2, HasFullAccess: false}, {ID: 3, HasFullAccess: true}}
	}
	hints := []peopleModule.SchoolDayHint{
		{StudentID: 1, Body: "Heute Sportfest", AuthorName: "Frau Kurz", SchoolClass: "2a"},
		{StudentID: 1, Body: "Hat Bauchweh", AuthorName: "Frau Kurz"},
	}

	directory := &teacherHintDirectory{hints: hints}
	rs := &Resource{ResourceConfig{PeopleDirectory: directory}}
	responses := page()
	require.NoError(t, rs.enrichWithTeacherHints(reading, responses, studentListPlan{planningDate: today, isToday: true}))
	assert.Equal(t, []TeacherHint{
		{Body: "Heute Sportfest", AuthorName: "Frau Kurz", SchoolClass: "2a"},
		{Body: "Hat Bauchweh", AuthorName: "Frau Kurz"},
	}, responses[0].TeacherHints)
	assert.Empty(t, responses[1].TeacherHints)
	assert.Equal(t, [][]int64{{1, 3}}, directory.asked, "a redacted row is never asked for")
	assert.Equal(t, []calendar.Date{today}, directory.dates)

	for name, call := range map[string]struct {
		ctx  context.Context
		plan studentListPlan
	}{
		"without users:read": {testpkg.WithPermissionsContext(context.Background(), []string{"students:read"}), studentListPlan{planningDate: today, isToday: true}},
		"another day":        {reading, studentListPlan{planningDate: today.AddDays(1)}},
	} {
		directory := &teacherHintDirectory{hints: hints}
		rs := &Resource{ResourceConfig{PeopleDirectory: directory}}
		responses := page()
		require.NoError(t, rs.enrichWithTeacherHints(call.ctx, responses, call.plan), name)
		for _, response := range responses {
			assert.Empty(t, response.TeacherHints, name)
		}
		assert.Empty(t, directory.asked, name)
	}

	failure := errors.New("directory unavailable")
	rs = &Resource{ResourceConfig{PeopleDirectory: &teacherHintDirectory{err: failure}}}
	err := rs.enrichWithTeacherHints(reading, page(), studentListPlan{planningDate: today, isToday: true})
	require.ErrorIs(t, err, failure)
}
