package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The room filters find "Raum Süd" for "raum sud" (#4949).
func TestRoomSearchIgnoresAccents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	wanted := testpkg.CreateTestRoom(t, db, "Raum Süd Łąka")
	testpkg.CreateTestRoom(t, db, "Raum Nord")
	rooms, err := NewUnobserved(db)
	require.NoError(t, err)

	query := "raum sud laka"
	for name, filter := range map[string]facilities.RoomFilter{
		"name contains": {NameContains: &query},
		"search":        {Search: &query},
	} {
		listed, err := rooms.ListRooms(testpkg.Ctx(t), filter)
		require.NoError(t, err, name)
		require.Len(t, listed, 1, name)
		assert.Equal(t, wanted.ID, listed[0].ID, name)
	}
}
