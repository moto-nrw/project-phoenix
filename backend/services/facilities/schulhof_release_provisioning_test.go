package facilities

// Provisioning behavior of the room release ("offener Raum", #3064): a newly
// provisioned school starts with the same usable yard the migration gives
// existing ones, and an administrator's explicit deactivation survives every
// later provisioning run.

import (
	"context"
	"testing"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newReleaseTestService wires the compatibility service with the collaborators
// these tests do not exercise stubbed out to their empty answers.
func newReleaseTestService(owner facilitiesModule.Capability) Service {
	return NewServiceWithConfig(ServiceConfig{
		Rooms: owner,
		Occupancy: func(_ context.Context, rooms []facilitiesModule.Room) ([]RoomWithOccupancy, error) {
			result := make([]RoomWithOccupancy, 0, len(rooms))
			for index := range rooms {
				result = append(result, RoomWithOccupancy{Room: &rooms[index]})
			}
			return result, nil
		},
		History: func(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionEntry, error) {
			return nil, nil
		},
		ValidateDeletion: func(context.Context, int64) error { return nil },
	})
}

func TestSchulhofProvisioningReleasesTheNewRoom(t *testing.T) {
	t.Parallel()

	rooms, activities := &roomServiceStub{}, &activityCatalogStub{}
	service := NewSchulhofService(rooms, activities, openGroupCatalogStub{}, nil)

	_, err := service.EnsureInfrastructure(context.Background(), 0)

	require.NoError(t, err)
	require.NotNil(t, rooms.created)
	assert.Equal(t, facilitiesModule.SchulhofRoomName, rooms.created.Name)
	assert.True(t, rooms.created.IsSystem)
	assert.True(t, rooms.created.IsOpenRoom,
		"a newly provisioned school must start with a usable yard, like existing ones after the migration")
}

// TestWCProvisioningLeavesTheRoomUnreleased is the counterpart: the toilets are
// short-stay kiosk infrastructure and must never become a released destination.
func TestWCProvisioningLeavesTheRoomUnreleased(t *testing.T) {
	t.Parallel()

	rooms, activities := &roomServiceStub{}, &activityCatalogStub{}
	service := NewWCService(rooms, activities, nil)

	_, err := service.EnsureInfrastructure(context.Background())

	require.NoError(t, err)
	require.NotNil(t, rooms.created)
	assert.Equal(t, facilitiesModule.WCRoomName, rooms.created.Name)
	assert.False(t, rooms.created.IsOpenRoom,
		"the toilet room must never be released as an open room")
}

// TestSchulhofProvisioningPreservesAnExplicitDeactivation is the acceptance
// criterion that a restart, a deployment or a later provisioning run must not
// undo an administrator's decision. The room already exists, so provisioning
// returns it untouched instead of re-stamping the release.
func TestSchulhofProvisioningPreservesAnExplicitDeactivation(t *testing.T) {
	t.Parallel()

	roomID := int64(41)
	deactivated := &facilitiesModule.Room{
		ID:         roomID,
		Name:       facilitiesModule.SchulhofRoomName,
		IsSystem:   true,
		IsOpenRoom: false,
	}
	rooms := &roomServiceStub{schulhof: deactivated}
	activities := &activityCatalogStub{
		activities: []SystemActivity{{
			ID: 71, Name: facilitiesModule.SchulhofActivityName,
			PlannedRoomID: &roomID, IsSystem: true,
		}},
	}
	service := NewSchulhofService(rooms, activities, openGroupCatalogStub{}, nil)

	_, err := service.EnsureInfrastructure(context.Background(), 0)

	require.NoError(t, err)
	assert.Nil(t, rooms.created,
		"an existing yard must not be re-created by provisioning")
	assert.False(t, deactivated.IsOpenRoom,
		"provisioning must not re-release a yard the administration switched off")
}

// TestSchulhofProvisioningPreservesAnExistingRelease is the mirror: a released
// yard stays released, so a provisioning run is a no-op either way.
func TestSchulhofProvisioningPreservesAnExistingRelease(t *testing.T) {
	t.Parallel()

	roomID := int64(41)
	released := &facilitiesModule.Room{
		ID:         roomID,
		Name:       facilitiesModule.SchulhofRoomName,
		IsSystem:   true,
		IsOpenRoom: true,
	}
	rooms := &roomServiceStub{schulhof: released}
	activities := &activityCatalogStub{
		activities: []SystemActivity{{
			ID: 71, Name: facilitiesModule.SchulhofActivityName,
			PlannedRoomID: &roomID, IsSystem: true,
		}},
	}
	service := NewSchulhofService(rooms, activities, openGroupCatalogStub{}, nil)

	_, err := service.EnsureInfrastructure(context.Background(), 0)

	require.NoError(t, err)
	assert.Nil(t, rooms.created)
	assert.True(t, released.IsOpenRoom)
}

// TestCompatibilityServiceCarriesTheReleaseToTheOwner covers the legacy bridge
// both provisioning paths write through: the release has to survive the hop
// from the compatibility service to the Facilities owner.
func TestCompatibilityServiceCarriesTheReleaseToTheOwner(t *testing.T) {
	t.Parallel()

	owner := &ownerStub{room: facilitiesModule.Room{ID: 41, Name: facilitiesModule.SchulhofRoomName}}
	service := newReleaseTestService(owner)

	require.NoError(t, service.CreateRoom(context.Background(), &facilitiesModule.Room{
		Name: facilitiesModule.SchulhofRoomName, IsOpenRoom: true,
	}))
	assert.True(t, owner.created.IsOpenRoom)

	require.NoError(t, service.CreateRoom(context.Background(), &facilitiesModule.Room{
		Name: "Werkraum",
	}))
	assert.False(t, owner.created.IsOpenRoom,
		"an ordinary room is created unreleased")
}
