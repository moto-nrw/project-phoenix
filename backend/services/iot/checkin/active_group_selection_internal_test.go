package checkin

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUseExistingActiveGroupCannotSelectPreviousDaySession(t *testing.T) {
	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, timezone.Berlin)
	deviceID := int64(91)
	room := &facilities.Room{Model: base.Model{ID: 42}, Name: "Schulhof"}
	stale := &active.Group{
		Model:     base.Model{ID: 100},
		RoomID:    room.ID,
		DeviceID:  &deviceID,
		StartTime: now.AddDate(0, 0, -1),
	}
	current := &active.Group{
		Model:     base.Model{ID: 101},
		RoomID:    room.ID,
		StartTime: now.Add(-time.Hour),
	}

	groups := activeGroupsStartedToday([]*active.Group{stale, current}, now)
	selection := (&CheckinService{}).useExistingActiveGroup(context.Background(), groups, room, deviceID)

	require.NotNil(t, selection)
	assert.Equal(t, current.ID, selection.Group.ID)
	assert.False(t, selection.DeviceScoped, "yesterday's device match must not win")
}
