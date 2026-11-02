//go:build testhooks

package httpadapter

import (
	"testing"

	"github.com/go-chi/chi/v5"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// RoomMetadataBudgetRouterForTest exposes the existing real root-bound fixture
// to the external metadata budget tests, without copying its composition.
func RoomMetadataBudgetRouterForTest(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()
	tc := setupRoomsRoute(t)
	return tc.db, tc.router
}

// RoomHistoryBudgetRouterForTest enables the established history fixture
// through the root-bound SettingsWriter and returns the protected router.
// Reuse this package's existing registry-contract constants: external adapter
// tests must not depend on the Settings Platform port package.
func RoomHistoryBudgetRouterForTest(t *testing.T) (*bun.DB, chi.Router) {
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
