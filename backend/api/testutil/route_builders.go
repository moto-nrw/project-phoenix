package testutil

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// Route-sized setup names keep API tests coupled to the route family they
// exercise. Their implementation remains on the legacy graph until the
// capability-specific builders land; SetupAPITest can then be deleted once
// these adapters have no callers.
func SetupActiveRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupActivitiesRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupAuthRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupClassDayRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupClassListEntriesRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupConfigRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupFeedbackRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupFileStoreRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupGradeTransitionsRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupGroupsRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupGuardiansRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupImportRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupIoTAttendanceRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupIoTCheckinRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupIoTDataRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupIoTDevicesRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupIoTSessionsRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupIoTStaffClockRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupOperatorSettingsRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupRoomsRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupSchedulesRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupShiftTypesRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupSSERoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }
func SetupUserContextRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
func SetupUsersRoute(t *testing.T) (*bun.DB, *services.Factory) { t.Helper(); return SetupAPITest(t) }

func SetupBirthdaysRoute(t *testing.T, clocks ...func() time.Time) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t, clocks...)
}
func SetupSchoolRoute(t *testing.T, clocks ...func() time.Time) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t, clocks...)
}
func SetupStaffRoute(t *testing.T, clocks ...func() time.Time) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t, clocks...)
}
func SetupStatisticsRoute(t *testing.T, clocks ...func() time.Time) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t, clocks...)
}
func SetupStudentsRoute(t *testing.T, clocks ...func() time.Time) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t, clocks...)
}
func SetupTimetableRoute(t *testing.T) (*bun.DB, *services.Factory) {
	t.Helper()
	return SetupAPITest(t)
}
