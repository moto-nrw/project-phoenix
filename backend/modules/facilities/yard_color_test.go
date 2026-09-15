package facilities_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/stretchr/testify/require"
)

type yardColorQuery struct {
	facilities.Query
	list func(context.Context, facilities.RoomFilter) ([]facilities.Room, error)
}

func (q yardColorQuery) ListRooms(ctx context.Context, filter facilities.RoomFilter) ([]facilities.Room, error) {
	return q.list(ctx, filter)
}

func TestSchulhofRoomColorSelectsCanonicalSystemRoom(t *testing.T) {
	t.Parallel()
	color, empty := "#A3D977", ""
	canonical := facilities.Room{Name: facilities.SchulhofRoomName, IsSystem: true, Color: &color}
	legacy := facilities.Room{Name: "schulhof", Color: &color}
	for _, tc := range []struct {
		name  string
		rooms []facilities.Room
		want  *string
	}{
		{"configured", []facilities.Room{canonical}, &color},
		{"missing", nil, nil},
		{"legacy only", []facilities.Room{legacy}, nil},
		{"ordinary room", []facilities.Room{{Name: facilities.SchulhofRoomName, Color: &color}}, nil},
		{"legacy first", []facilities.Room{legacy, canonical}, &color},
		{"canonical first", []facilities.Room{canonical, legacy}, &color},
		{"unset", []facilities.Room{{Name: facilities.SchulhofRoomName, IsSystem: true}}, nil},
		{"empty", []facilities.Room{{Name: facilities.SchulhofRoomName, IsSystem: true, Color: &empty}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			calls := 0
			query := yardColorQuery{list: func(got context.Context, filter facilities.RoomFilter) ([]facilities.Room, error) {
				calls++
				require.Equal(t, ctx, got)
				require.Equal(t, facilities.RoomFilter{Name: &canonical.Name}, filter)
				return tc.rooms, nil
			}}
			got, err := facilities.SchulhofRoomColor(ctx, query)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, 1, calls)
		})
	}
}

func TestSchulhofRoomColorPreservesLookupErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("lookup failed")
	query := yardColorQuery{list: func(context.Context, facilities.RoomFilter) ([]facilities.Room, error) { return nil, boom }}
	color, err := facilities.SchulhofRoomColor(t.Context(), query)
	require.Nil(t, color)
	require.ErrorIs(t, err, boom)
	require.EqualError(t, err, "list Schulhof rooms: lookup failed")
	color, err = facilities.SchulhofRoomColor(t.Context(), nil)
	require.Nil(t, color)
	require.NoError(t, err)
}
