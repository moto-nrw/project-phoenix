package facilities

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsReservedRoomColor(t *testing.T) {
	t.Parallel()
	reserved := []string{
		"#83CD2D", "#5080D8", "#FF3130", "#F78C10", "#D946EF",
		"#EAB308", "#7C3AED", "#6B7280", "#4F46E5",
	}
	for _, color := range reserved {
		variants := []string{color, strings.ToLower(color), " " + color + " ", strings.TrimPrefix(color, "#")}
		for _, variant := range variants {
			assert.True(t, IsReservedRoomColor(variant), "expected %q to be reserved", variant)
		}
	}
	for _, color := range []string{"#A3D977", "#FFD580", "#1ABC9C", "#000000", "#FFFFFF"} {
		assert.False(t, IsReservedRoomColor(color), "%s should not be reserved", color)
	}
	for _, color := range []string{"", "#", "not-a-color", "#GGGGGG"} {
		assert.False(t, IsReservedRoomColor(color), "%q should not be reserved", color)
	}
}
