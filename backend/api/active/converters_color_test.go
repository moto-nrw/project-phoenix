package active

import (
	"encoding/json"
	"testing"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewActiveGroupResponse_RoomColorPropagation guards the most fragile
// link in the badge-color pipeline: the RoomSimple struct used by every
// active-group response. The frontend BFFs decode this JSON to populate
// `current_room_color` for badges. If anyone:
//   - removes Color from RoomSimple,
//   - forgets to set it in newActiveGroupResponse, or
//   - changes the JSON tag from "color" to something else,
//
// every per-room badge color silently disappears with no compile error.
//
// This test marshals the response struct and inspects the raw JSON to catch
// all three regressions in one shot.
func TestNewActiveGroupResponse_RoomColorPropagation(t *testing.T) {
	t.Parallel()

	now := time.Now()
	color := "#A3D977"

	t.Run("color is included in JSON when room has color set", func(t *testing.T) {
		group := &activeModels.Group{
			Model:     modelBase.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
			RoomID:    10,
			StartTime: now,
			Room: &facilities.Room{
				ID: 10, Name: "Werkraum", Color: &color,
			},
		}

		resp := newActiveGroupResponse(group)
		require.NotNil(t, resp.Room, "Room must be populated when group.Room is set")

		// Direct field check — catches "Color field removed" regression.
		require.NotNil(t, resp.Room.Color)
		assert.Equal(t, "#A3D977", *resp.Room.Color)

		// JSON-shape check — catches "json tag renamed" regression. The
		// frontend BFF reads this exact key, so a tag rename would silently
		// drop the value.
		raw, err := json.Marshal(resp.Room)
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":10,"name":"Werkraum","color":"#A3D977"}`, string(raw))
	})

	t.Run("color is omitted from JSON when room has no color", func(t *testing.T) {
		// When color is nil the response must still be valid JSON without
		// a "color" key — the omitempty tag earns its keep here. Without
		// it, the field would serialize as "color":null, which the
		// frontend would treat as an explicit cleared value (still falls
		// back to blue, but it's a noisier wire payload).
		group := &activeModels.Group{
			Model:     modelBase.Model{ID: 1},
			RoomID:    10,
			StartTime: now,
			Room: &facilities.Room{
				ID: 10, Name: "Bibliothek",
			},
		}

		resp := newActiveGroupResponse(group)
		require.NotNil(t, resp.Room)
		assert.Nil(t, resp.Room.Color)

		raw, err := json.Marshal(resp.Room)
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":10,"name":"Bibliothek"}`, string(raw),
			"omitempty must drop the color key when not set")
	})
}
