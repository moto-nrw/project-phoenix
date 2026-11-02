//go:build testhooks

package httpadapter

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
)

// RoomMetadataBudgetRouterForTest exposes the existing real root-bound fixture
// to the external metadata budget tests, without copying its composition.
func RoomMetadataBudgetRouterForTest(t *testing.T) (*bun.DB, chi.Router) {
	t.Helper()
	tc := setupRoomsRoute(t)
	return tc.db, tc.router
}
