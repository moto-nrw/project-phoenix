// Router-level tests for the time a holiday guest of the hosting school is
// in the school's lists (#5630, ADR 0120 Nachtrag): from the first day of its
// week to its last, never before, while the school's own children with a
// later start stay as they are.
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestHolidayGuestsAppearInTheSchoolListsOnlyDuringTheirWeek(t *testing.T) {
	t.Parallel()
	f := newHolidayGuestFixture(t, "Träger Feriengäste Zeitraum")
	week := f.offer.Weeks[0].ID // 06.07.2099 to 10.07.2099
	lead := f.staffOf(t, f.nord, permissions.ConfigRead, permissions.UsersCreate, permissions.UsersRead)

	// An own child of the OGS Nord whose care starts after the holidays: it
	// waits like a guest before its week and stays in the lists as before.
	later := testpkg.CreateTestStudentForTenant(t, f.db, f.nord, "Nora", "Später", "1a")
	_, err := f.db.ExecContext(t.Context(), `UPDATE users.student_school_memberships SET status = 'pending', enrolled_from = '2099-08-15' WHERE student_profile_id = ? AND deleted_at IS NULL`, later.ID)
	require.NoError(t, err)

	f.today.Store(calendar.NewDate(2099, 7, 1))
	f.confirm(t, f.siblings, "confirm")
	require.Equal(t, http.StatusOK, f.assign(t, lead, week, f.group).Code)

	before := calendar.NewDate(2099, 7, 3)
	f.today.Store(before)
	f.reconcile(t)
	roster, directory, picker := f.lists(t, before)
	assert.NotContains(t, roster, "Ben Muster", "before its week the guest is not in the tablet list or the bracelet assignment")
	assert.NotContains(t, directory, "Ben Muster", "before its week the guest is not in the child list")
	assert.NotContains(t, picker, "Ben Muster", "before its week the guest is not in the calendar picker")
	assert.Contains(t, roster, "Nora Später", "an own child with a later start stays in the live roster")
	assert.Contains(t, directory, "Nora Später", "an own child with a later start stays in the child list")

	first := calendar.NewDate(2099, 7, 6)
	f.today.Store(first)
	f.reconcile(t)
	roster, directory, picker = f.lists(t, first)
	assert.Contains(t, roster, "Ben Muster", "from the first day the guest is in the live roster")
	assert.Contains(t, directory, "Ben Muster", "from the first day the guest is in the child list")
	assert.Contains(t, picker, "Ben Muster", "from the first day the guest is in the calendar picker")

	after := calendar.NewDate(2099, 7, 11)
	f.today.Store(after)
	roster, directory, picker = f.lists(t, after)
	assert.NotContains(t, roster, "Ben Muster", "from the day after the week the guest is in no live list")
	assert.NotContains(t, directory, "Ben Muster", "from the day after the week the guest is not in the child list")
	assert.NotContains(t, picker, "Ben Muster", "from the day after the week the guest is not in the calendar picker")
	assert.Contains(t, directory, "Nora Später")
}
