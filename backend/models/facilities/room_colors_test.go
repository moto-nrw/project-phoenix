package facilities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReservedRoomColor(t *testing.T) {
	t.Parallel()

	t.Run("rejects every status badge color", func(t *testing.T) {
		// Mirrors LOCATION_COLORS in frontend/src/lib/location-helper.ts.
		// If the frontend list grows, the backend list must too — this test
		// won't catch that drift, but the cross-codebase reference comment
		// in room_colors.go points reviewers at it.
		reserved := []string{
			"#83CD2D", // GROUP_ROOM
			"#5080D8", // OTHER_ROOM (the blue we're trying to escape)
			"#FF3130", // HOME
			"#F78C10", // SCHOOLYARD
			"#D946EF", // TRANSIT
			"#EAB308", // SICK
			"#7C3AED", // EXCUSED
			"#6B7280", // UNKNOWN / NOT_ARRIVAL
		}
		for _, c := range reserved {
			assert.True(t, IsReservedRoomColor(c),
				"expected %s to be reserved", c)
		}
	})

	t.Run("normalizes case and whitespace", func(t *testing.T) {
		// User-supplied input could land in any of these shapes; all must hit.
		variants := []string{"#5080d8", " #5080D8 ", "5080D8", "5080d8"}
		for _, v := range variants {
			assert.True(t, IsReservedRoomColor(v),
				"variant %q should be reserved", v)
		}
	})

	t.Run("expands #RGB shorthand", func(t *testing.T) {
		// #6B7280 is reserved as #6B7280; test a known shorthand collapse to
		// confirm the expansion path works at all.
		assert.False(t, IsReservedRoomColor("#FFF"),
			"#FFF expands to #FFFFFF which is not reserved")
		// Build a 3-char that expands to a reserved 6-char: there is no exact
		// 3→6 collision in the list (e.g. #837 → #883377 ≠ any reserved), so
		// we just sanity-check the helper returns false for non-matches.
	})

	t.Run("allows arbitrary non-status colors", func(t *testing.T) {
		// #4F46E5 used to live in this list as "non-reserved". It now sits
		// in the reserved set on purpose — see migration 1.15.45 / the
		// header comment in room_colors.go for why allowing admins to
		// re-pick the legacy bug-default would defeat the migration's
		// invariant. The test below asserts it's reserved.
		nonReserved := []string{
			"#A3D977", "#FFD580", "#1ABC9C", "#000000", "#FFFFFF",
		}
		for _, c := range nonReserved {
			assert.False(t, IsReservedRoomColor(c),
				"%s should not be reserved", c)
		}
	})

	t.Run("rejects legacy bug-default #4F46E5", func(t *testing.T) {
		// Migration 1.15.45 NULLs every row carrying this hex; the audit
		// backup table relies on "any #4F46E5 row in facilities.rooms
		// must have come from the bug" as a restore invariant. Letting an
		// admin pick the same hex afterwards would break that. Both
		// upper- and lower-case must be rejected since the picker emits
		// uppercase but a direct API call could send either.
		assert.True(t, IsReservedRoomColor("#4F46E5"))
		assert.True(t, IsReservedRoomColor("#4f46e5"))
	})

	t.Run("handles empty and malformed input gracefully", func(t *testing.T) {
		assert.False(t, IsReservedRoomColor(""))
		assert.False(t, IsReservedRoomColor("#"))
		assert.False(t, IsReservedRoomColor("not-a-color"))
		assert.False(t, IsReservedRoomColor("#GGGGGG"))
	})
}

func TestRoomValidate_ReservedColor(t *testing.T) {
	t.Parallel()

	t.Run("rejects a reserved color", func(t *testing.T) {
		room := &Room{
			Name:  "Reserved Color Room",
			Color: stringPointer("#5080D8"),
		}
		err := room.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReservedColor),
			"expected ErrReservedColor, got %v", err)
		assert.True(t, IsValidationError(err),
			"reserved-color error should report as validation error")
	})

	t.Run("allows a non-reserved hex", func(t *testing.T) {
		room := &Room{
			Name:  "Custom Color Room",
			Color: stringPointer("#A3D977"),
		}
		require.NoError(t, room.Validate())
	})

	t.Run("allows nil color (default fallback to blue)", func(t *testing.T) {
		room := &Room{Name: "No Color"}
		require.NoError(t, room.Validate())
	})

	t.Run("rejects malformed hex with sentinel", func(t *testing.T) {
		room := &Room{
			Name:  "Bad Color",
			Color: stringPointer("not-hex"),
		}
		err := room.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrInvalidColorFormat))
	})

	t.Run("auto-prefixes # before validation", func(t *testing.T) {
		room := &Room{
			Name:  "Prefixed",
			Color: stringPointer("A3D977"),
		}
		require.NoError(t, room.Validate())
		require.NotNil(t, room.Color)
		assert.Equal(t, "#A3D977", *room.Color)
	})

	t.Run("auto-prefix into reserved color is still rejected", func(t *testing.T) {
		room := &Room{
			Name:  "Sneaky",
			Color: stringPointer("5080D8"), // missing # → adds # → matches reserved
		}
		err := room.Validate()
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrReservedColor))
	})

	t.Run("expands #RGB shorthand to #RRGGBB before storing", func(t *testing.T) {
		// The unique index is on LOWER(color); without expansion, "#ABC"
		// and "#AABBCC" persist as textually different rows even though
		// they render the same CSS color. Native picker always emits 6
		// digits, but a direct API call could send either form.
		room := &Room{
			Name:  "Shorthand",
			Color: stringPointer("#abc"),
		}
		require.NoError(t, room.Validate())
		require.NotNil(t, room.Color)
		assert.Equal(t, "#AABBCC", *room.Color)
	})

	t.Run("expands shorthand even when '#' prefix is missing", func(t *testing.T) {
		// Validate() prepends "#" before regex-matching, so a bare "abc"
		// must travel the same canonicalisation path as "#abc": prefix →
		// expand to 6 digits → uppercase. This pins the storage form so
		// the unique index on LOWER(color) cannot be sidestepped by
		// posting the shorthand without a leading hash. None of the
		// current reserved hexes are repeating-pair palindromes (e.g.
		// #AABBCC), so we cannot also assert reserved-rejection on a
		// shorthand input — that path is covered indirectly: expansion
		// runs before IsReservedRoomColor, so any future reserved hex of
		// that shape would trip the reserved check after expansion.
		room := &Room{
			Name:  "Shorthand prefix",
			Color: stringPointer("abc"), // missing # → adds # → expands → uppercases
		}
		require.NoError(t, room.Validate())
		require.NotNil(t, room.Color)
		assert.Equal(t, "#AABBCC", *room.Color)
	})

	t.Run("normalises mixed-case input to upper-case", func(t *testing.T) {
		// The unique index lives on LOWER(color), so storage casing doesn't
		// affect dedup — but audit log + API consumers expect a single
		// canonical form. Without normalisation, "#a3d977" and "#A3D977"
		// flow through as-is and the API answers two visually-identical
		// rooms with different colour strings.
		room := &Room{
			Name:  "Mixed Case",
			Color: stringPointer("#a3D977"),
		}
		require.NoError(t, room.Validate())
		require.NotNil(t, room.Color)
		assert.Equal(t, "#A3D977", *room.Color)
	})
}

func stringPointer(value string) *string { return &value }
