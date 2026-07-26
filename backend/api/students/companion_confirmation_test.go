package students

import (
	"testing"

	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/require"
)

// TestCompanionConflictsConfirmed pins what a confirmed extension is worth: the
// user answered a question about NAMED children on NAMED weekdays, and the rows
// are unlocked while they answer. A conflict that appeared (or grew a day) in
// the meantime was never on screen, so it must not ride along on that yes —
// the caller asks again instead (#1694).
func TestCompanionConflictsConfirmed(t *testing.T) {
	const (
		tom = int64(77)
		mia = int64(88)
	)

	t.Run("exactly what was confirmed", func(t *testing.T) {
		require.True(t, companionConflictsConfirmed(
			[]usersService.CompanionConflict{{StudentID: tom, Weekdays: []string{"thu"}}},
			[]CompanionEntry{{CompanionStudentID: tom, Weekdays: []string{"thu"}}},
		))
	})

	t.Run("a companion that appeared afterwards is not confirmed", func(t *testing.T) {
		require.False(t, companionConflictsConfirmed(
			[]usersService.CompanionConflict{
				{StudentID: tom, Weekdays: []string{"thu"}},
				{StudentID: mia, Weekdays: []string{"thu"}},
			},
			[]CompanionEntry{{CompanionStudentID: tom, Weekdays: []string{"thu"}}},
		))
	})

	t.Run("a weekday that appeared afterwards is not confirmed", func(t *testing.T) {
		require.False(t, companionConflictsConfirmed(
			[]usersService.CompanionConflict{{StudentID: tom, Weekdays: []string{"thu", "fri"}}},
			[]CompanionEntry{{CompanionStudentID: tom, Weekdays: []string{"thu"}}},
		))
	})

	t.Run("a conflict that resolved itself leaves the rest confirmed", func(t *testing.T) {
		require.True(t, companionConflictsConfirmed(
			[]usersService.CompanionConflict{{StudentID: tom, Weekdays: []string{"thu"}}},
			[]CompanionEntry{
				{CompanionStudentID: tom, Weekdays: []string{"thu"}},
				{CompanionStudentID: mia, Weekdays: []string{"thu"}},
			},
		))
	})

	t.Run("a flag without a list confirms nothing", func(t *testing.T) {
		require.False(t, companionConflictsConfirmed(
			[]usersService.CompanionConflict{{StudentID: tom, Weekdays: []string{"thu"}}},
			nil,
		))
	})
}
