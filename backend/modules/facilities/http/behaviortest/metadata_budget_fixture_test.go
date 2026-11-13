package httpadapter_test

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// roomMetadataBudgetRouter serves the metadata budget tests the real
// root-bound fixture, without copying its composition.
func roomMetadataBudgetRouter(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()
	tc := setupRoomsRoute(t)
	return tc.db, tc.router
}

// roomHistoryBudgetRouter enables the established history fixture
// through the root-bound SettingsWriter and returns the protected router.
// Reuse this package's existing registry-contract constants: external adapter
// tests must not depend on the Settings Platform port package.
func roomHistoryBudgetRouter(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()
	tc := setupRoomsRoute(t)
	ctx := testpkg.Ctx(t)
	if err := tc.services.Settings.SetValue(ctx, settingAttendanceLogEnabled, true, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := tc.services.Settings.SetValue(ctx, settingRoomDetailVisibleDays, 2, nil, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tc.services.Settings.ResetValue(ctx, settingRoomDetailVisibleDays, nil, nil); err != nil {
			t.Error(err)
		}
		if err := tc.services.Settings.ResetValue(ctx, settingAttendanceLogEnabled, nil, nil); err != nil {
			t.Error(err)
		}
	})
	return tc.db, tc.router
}
