package compose_test

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSickReportLocksStaffRowsInOneQuery(t *testing.T) {
	t.Parallel()
	e := buildSickCascadeEnv(t)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	origin := e.createShift(t, e.subject.ID, stampWeekOne, "08:00", "10:00", func(s *workforce.StaffShift) {
		s.SickAbsenceID = &absenceID
	})
	for _, id := range []int64{e.sub.ID, e.admin.ID} {
		e.createShift(t, id, stampWeekOne, "08:00", "10:00", func(s *workforce.StaffShift) {
			s.OriginShiftID = &origin.ID
		})
	}
	counter := testpkg.CaptureQueriesForContext(t, e.db)
	e.inTx(t, func(ctx context.Context) error {
		return e.cascade.LockSickReportWrite(counter.Context(ctx), e.subject.ID, absenceID, stampWeekOne.String(), stampWeekOne.String())
	})
	locks := counter.Matching(func(sql string) bool {
		return strings.Contains(sql, "users.staff_school_memberships") && strings.Contains(sql, "for no key update")
	})
	require.Len(t, locks, 1, "subject and cover owners share one ordered membership lock query")
	require.Contains(t, strings.ToLower(locks[0]), `order by "staff".id asc`)
	require.Empty(t, counter.Matching(func(sql string) bool {
		return strings.Contains(sql, "users.staff_employment_profiles")
	}), "locking memberships needs no employment data")
}
