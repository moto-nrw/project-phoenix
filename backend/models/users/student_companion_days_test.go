package users

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStudentValidate_AccompaniedCoverIsPerWeekday pins the rule a single
// "has links" flag used to get wrong (#1694): a structured companion link
// answers "mit wem" for the weekdays it covers and for no others. A Monday
// Laufgemeinschaft says nothing about who the child leaves with on Tuesday, so
// an accompanied Tuesday still needs its own answer — a link on that day or the
// free-text note.
func TestStudentValidate_AccompaniedCoverIsPerWeekday(t *testing.T) {
	t.Parallel()

	newStudent := func() *Student {
		student := &Student{SchoolClass: "1a"}
		student.PersonID = 42
		student.AllowedDepartureModes = AllowedDepartureModes{
			PickupDayMonday:  []DepartureMode{DepartureAccompanied},
			PickupDayTuesday: []DepartureMode{DepartureAccompanied},
		}
		return student
	}

	t.Run("link on one of two accompanied days is rejected", func(t *testing.T) {
		student := newStudent()
		student.MarkDepartureCompanionDays(PickupDayMonday)

		require.True(t, errors.Is(student.Validate(), ErrDepartureCompanionNoteRequired))
	})

	t.Run("link on every accompanied day is accepted", func(t *testing.T) {
		student := newStudent()
		student.MarkDepartureCompanionDays(PickupDayMonday, PickupDayTuesday)

		require.NoError(t, student.Validate())
	})

	t.Run("free-text note covers every day on its own", func(t *testing.T) {
		student := newStudent()
		note := "Nachbarskind"
		student.DepartureCompanionNote = &note

		require.NoError(t, student.Validate())
	})

	t.Run("no cover at all is rejected", func(t *testing.T) {
		require.True(t, errors.Is(newStudent().Validate(), ErrDepartureCompanionNoteRequired))
	})

	t.Run("legacy departure_days is covered per weekday too", func(t *testing.T) {
		student := &Student{SchoolClass: "1a"}
		student.PersonID = 42
		student.DepartureDays = DepartureDays{
			PickupDayMonday:  DepartureAccompanied,
			PickupDayFriday:  DepartureAccompanied,
			PickupDayTuesday: DepartureBus,
		}
		student.MarkDepartureCompanionDays(PickupDayMonday)

		require.True(t, errors.Is(student.Validate(), ErrDepartureCompanionNoteRequired))

		student.MarkDepartureCompanionDays(PickupDayFriday)
		require.NoError(t, student.Validate())
	})
}

// TestCompanionDaysFromLinks folds the submitted list into the cover set the
// model checks against, which is what lets a caller vouch for edges it is about
// to write in the same transaction.
func TestCompanionDaysFromLinks(t *testing.T) {
	t.Parallel()

	days := CompanionDaysFromLinks([]CompanionLink{
		{CompanionStudentID: 11, Weekdays: []string{PickupDayMonday}},
		{CompanionStudentID: 12, Weekdays: []string{PickupDayMonday, PickupDayThursday}},
	})

	require.Equal(t, map[string]bool{PickupDayMonday: true, PickupDayThursday: true}, days)
	require.Empty(t, CompanionDaysFromLinks(nil))
}
