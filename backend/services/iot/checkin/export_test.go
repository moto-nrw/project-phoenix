package checkin

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// Test-only accessors that expose unexported CheckinService internals to the
// external checkin_test package. This is the standard Go export_test.go bridge:
// it lives in package checkin but is compiled only into the test binary, and it
// imports nothing that would re-create the services ↔ services/iot/checkin
// import cycle (unlike importing the services factory, which the external
// checkin_test package does instead).

func (s *CheckinService) CountActiveGroupOccupancyForTest(ctx context.Context, activeGroupID int64) (int, error) {
	return s.countActiveGroupOccupancy(ctx, activeGroupID)
}

func (s *CheckinService) RoomNameByIDForTest(ctx context.Context, room *facilities.Room, roomID int64) string {
	return s.roomNameByID(ctx, room, roomID)
}

func (s *CheckinService) RoomNameForResponseForTest(ctx context.Context, currentVisit *active.Visit, roomID *int64) string {
	return s.roomNameForResponse(ctx, currentVisit, roomID)
}

func (s *CheckinService) EnsureWCRoomForTest(ctx context.Context) (*facilities.Room, error) {
	return s.ensureWCRoom(ctx)
}

func (s *CheckinService) EnsureWCCategoryForTest(ctx context.Context) (*activities.Category, error) {
	return s.ensureWCCategory(ctx)
}

func (s *CheckinService) WCActivityGroupForTest(ctx context.Context) (*activities.Group, error) {
	return s.wcActivityGroup(ctx)
}

func (s *CheckinService) EnsureSchulhofRoomForTest(ctx context.Context) (*facilities.Room, error) {
	return s.ensureSchulhofRoom(ctx)
}

func (s *CheckinService) EnsureSchulhofCategoryForTest(ctx context.Context) (*activities.Category, error) {
	return s.ensureSchulhofCategory(ctx)
}

func (s *CheckinService) SchulhofActivityGroupForTest(ctx context.Context) (*activities.Group, error) {
	return s.schulhofActivityGroup(ctx)
}
