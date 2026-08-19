package active

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithYardRoomColor pins the binary-mode yard tinting introduced by
// #2405: the Schulhof room's colour reaches the badge, and only the yard
// state adopts it.
//
// Pure unit test — the resolver's own DB path is exercised through the
// snapshot integration tests; what needs pinning here is which of the three
// binary labels may carry a room colour.
func TestWithYardRoomColor(t *testing.T) {
	yard := "#A3D977"

	t.Run("stamps the color on the Schulhof label", func(t *testing.T) {
		out := withYardRoomColor(StudentLocationInfo{Location: YardLocationLabel}, &yard)
		if assert.NotNil(t, out.RoomColor) {
			assert.Equal(t, yard, *out.RoomColor)
		}
	})

	t.Run("leaves Anwesend untouched", func(t *testing.T) {
		// Binary mode has no room behind "Anwesend"; tinting it would colour
		// an unrelated badge with the yard's hex.
		out := withYardRoomColor(StudentLocationInfo{Location: "Anwesend"}, &yard)
		assert.Nil(t, out.RoomColor)
	})

	t.Run("leaves Abwesend untouched", func(t *testing.T) {
		out := withYardRoomColor(StudentLocationInfo{Location: "Abwesend"}, &yard)
		assert.Nil(t, out.RoomColor)
	})

	t.Run("passes through when no color is configured", func(t *testing.T) {
		// nil means "no colour set" — the frontend then renders the orange
		// Schulhof default.
		out := withYardRoomColor(StudentLocationInfo{Location: YardLocationLabel}, nil)
		assert.Nil(t, out.RoomColor)
	})
}

// TestResolveYardRoomColorWithoutCapability guards the optional-interface
// wiring: a Service implementation that predates YardRoomColorResolver must
// degrade to "no colour", never panic.
func TestResolveYardRoomColorWithoutCapability(t *testing.T) {
	assert.Nil(t, ResolveYardRoomColor(t.Context(), nil))
}

// yardColorRoomRepository is a stub for the single lookup GetSchulhofRoomColor
// performs; everything else on the interface stays unimplemented on purpose.
// rooms carries every case-insensitive name match, in the order the database
// would hand them over — the ordering is what the resolver must not trust.
type yardColorRoomRepository struct {
	facilityModels.RoomRepository
	rooms      []*facilityModels.Room
	err        error
	gotFilters map[string]any
}

func (r *yardColorRoomRepository) List(_ context.Context, filters map[string]any) ([]*facilityModels.Room, error) {
	r.gotFilters = filters
	return r.rooms, r.err
}

func yardColorRepo(rooms ...*facilityModels.Room) *yardColorRoomRepository {
	return &yardColorRoomRepository{rooms: rooms}
}

func yardColorService(repo facilityModels.RoomRepository) *service {
	return &service{ServiceDependencies: ServiceDependencies{RoomRepo: repo}}
}

func schulhofRoom(color *string) *facilityModels.Room {
	return &facilityModels.Room{
		Model:    base.Model{ID: 7},
		Name:     constants.SchulhofRoomName,
		IsSystem: true,
		Color:    color,
	}
}

// TestGetSchulhofRoomColor covers the resolver's own decision table: which
// lookup results count as "no colour" and which ones have to stay errors.
func TestGetSchulhofRoomColor(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the configured color", func(t *testing.T) {
		color := "#A3D977"
		repo := yardColorRepo(schulhofRoom(&color))

		got, err := yardColorService(repo).GetSchulhofRoomColor(ctx)

		require.NoError(t, err)
		if assert.NotNil(t, got) {
			assert.Equal(t, color, *got)
		}
		assert.Equal(t, map[string]any{"name": constants.SchulhofRoomName}, repo.gotFilters)
	})

	t.Run("treats a missing room as no color", func(t *testing.T) {
		// The Schulhof room is bootstrapped lazily, so a tenant that never
		// opened the yard is a normal state, not a failure.
		got, err := yardColorService(yardColorRepo()).GetSchulhofRoomColor(ctx)

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("propagates a real repository failure", func(t *testing.T) {
		// The review case: a dead connection must not read as "orange".
		boom := errors.New("connection refused")
		repo := &yardColorRoomRepository{err: &base.DatabaseError{Op: "list", Err: boom}}

		got, err := yardColorService(repo).GetSchulhofRoomColor(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, boom)
		assert.Nil(t, got)
	})

	t.Run("ignores a non-canonical or non-system room", func(t *testing.T) {
		// The name lookup matches case-insensitively; a stray "schulhof" room
		// from before the reservation guards must not tint the yard badge.
		color := "#A3D977"

		stray := schulhofRoom(&color)
		stray.Name = "schulhof"
		got, err := yardColorService(yardColorRepo(stray)).GetSchulhofRoomColor(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)

		notSystem := schulhofRoom(&color)
		notSystem.IsSystem = false
		got, err = yardColorService(yardColorRepo(notSystem)).GetSchulhofRoomColor(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("picks the canonical room out of a legacy collision", func(t *testing.T) {
		// The uniqueness index is case-sensitive, the lookup is not, so both
		// rows come back — and unordered. Whichever one the database hands
		// over first, the configured colour must survive.
		color := "#A3D977"
		legacy := schulhofRoom(nil)
		legacy.Model.ID = 3
		legacy.Name = "schulhof"
		legacy.IsSystem = false
		canonical := schulhofRoom(&color)

		for _, rooms := range [][]*facilityModels.Room{
			{legacy, canonical},
			{canonical, legacy},
		} {
			got, err := yardColorService(yardColorRepo(rooms...)).GetSchulhofRoomColor(ctx)
			require.NoError(t, err)
			if assert.NotNil(t, got) {
				assert.Equal(t, color, *got)
			}
		}
	})

	t.Run("treats an unset or empty color as no color", func(t *testing.T) {
		empty := ""
		for _, room := range []*facilityModels.Room{schulhofRoom(nil), schulhofRoom(&empty)} {
			got, err := yardColorService(yardColorRepo(room)).GetSchulhofRoomColor(ctx)
			require.NoError(t, err)
			assert.Nil(t, got)
		}
	})

	t.Run("degrades without a room repository", func(t *testing.T) {
		got, err := yardColorService(nil).GetSchulhofRoomColor(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("skips a nil row without an error", func(t *testing.T) {
		got, err := yardColorService(yardColorRepo(nil)).GetSchulhofRoomColor(ctx)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

// TestResolveYardRoomColorSuccess pins the happy path through the optional
// interface: a Service that resolves a colour hands it straight to the caller.
func TestResolveYardRoomColorSuccess(t *testing.T) {
	color := "#A3D977"
	svc := yardColorService(yardColorRepo(schulhofRoom(&color)))

	got := ResolveYardRoomColor(context.Background(), svc)

	if assert.NotNil(t, got) {
		assert.Equal(t, color, *got)
	}
}

// TestResolveYardRoomColorSwallowsErrors pins the fail-soft contract at the
// call site: the error is logged, the student list still renders.
func TestResolveYardRoomColorSwallowsErrors(t *testing.T) {
	repo := &yardColorRoomRepository{err: &base.DatabaseError{Op: "list", Err: errors.New("connection refused")}}

	assert.Nil(t, ResolveYardRoomColor(context.Background(), yardColorService(repo)))
}

// TestSnapshotBinaryYardColor covers the snapshot resolver end of the wiring
// without touching the database.
func TestSnapshotBinaryYardColor(t *testing.T) {
	yard := "#A3D977"
	const studentID int64 = 42

	snapshot := &StudentLocationSnapshot{
		Mode: PresenceModeBinary,
		Attendances: map[int64]*AttendanceStatus{
			studentID: {Status: "on_yard"},
		},
		YardRoomColor: &yard,
	}

	info := snapshot.ResolveStudentLocationWithTime(studentID, true)
	assert.Equal(t, YardLocationLabel, info.Location)
	if assert.NotNil(t, info.RoomColor) {
		assert.Equal(t, yard, *info.RoomColor)
	}

	// Same snapshot, a checked-in student: no room, no colour.
	snapshot.Attendances[studentID] = &AttendanceStatus{Status: "checked_in"}
	assert.Nil(t, snapshot.ResolveStudentLocationWithTime(studentID, true).RoomColor)
}
