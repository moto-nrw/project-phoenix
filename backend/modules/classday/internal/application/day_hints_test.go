package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const hintDay = calendar.Date("2099-03-02")

type openHintScope struct{}

func (openHintScope) SchoolMayWriteDayHints(context.Context) (bool, error) { return true, nil }

type fakeHintStore struct{ err error }

func (fakeHintStore) ListSchoolDayHints(context.Context, calendar.Date, []int64) ([]peopledirectory.SchoolDayHint, error) {
	return nil, nil
}

func (f fakeHintStore) SetSchoolDayHint(context.Context, peopledirectory.SetSchoolDayHint) error {
	return f.err
}

func (f fakeHintStore) ClearSchoolDayHint(context.Context, peopledirectory.ClearSchoolDayHint) error {
	return f.err
}

func newHintSeam(store fakeHintStore, announcer *countingAnnouncer) dayHints {
	return dayHints{
		store: store, scope: openHintScope{},
		rosters:   fakeDayRosters{roster: &DayRoster{Rows: []DayRosterRow{{StudentID: 11}, {StudentID: 12}}}},
		today:     func() calendar.Date { return hintDay },
		announcer: announcer,
	}
}

// Open OGS views refetch on the arrival-schedule event; a hint the teacher
// writes or withdraws must reach them without waiting for the next poll.
func TestDayHintWritesAnnounceToOGSViews(t *testing.T) {
	t.Parallel()

	announcer := &countingAnnouncer{}
	seam := newHintSeam(fakeHintStore{}, announcer)

	require.NoError(t, seam.SetDayHint(context.Background(), classday.DayHintWrite{
		SchoolClass: "3b", Date: classday.Date(hintDay), Body: "Heute Sportfest",
	}))
	require.NoError(t, seam.ClearDayHint(context.Background(), classday.DayHintTarget{
		SchoolClass: "3b", Date: classday.Date(hintDay), StudentID: 12,
	}))

	assert.Equal(t, 2, announcer.calls)
}

func TestFailedDayHintWritesStayQuiet(t *testing.T) {
	t.Parallel()

	announcer := &countingAnnouncer{}
	seam := newHintSeam(fakeHintStore{err: peopledirectory.ErrStudentNoteNotFound}, announcer)

	err := seam.ClearDayHint(context.Background(), classday.DayHintTarget{
		SchoolClass: "3b", Date: classday.Date(hintDay),
	})
	require.ErrorIs(t, err, classday.ErrDayHintNotFound)
	err = seam.SetDayHint(context.Background(), classday.DayHintWrite{
		SchoolClass: "3b", Date: classday.Date(hintDay), StudentID: 99, Body: "Bauchweh",
	})
	require.ErrorIs(t, err, classday.ErrDayHintStudentNotInClass)

	assert.Zero(t, announcer.calls)
}
