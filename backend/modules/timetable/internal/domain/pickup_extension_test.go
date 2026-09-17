package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenPickupExtensionBlocks(t *testing.T) {
	t.Parallel()
	freePlay := PickupExtensionBlock{ID: 1, StartTime: "14:45", EndTime: "16:00"}
	homework := PickupExtensionBlock{ID: 2, StartTime: "15:00", EndTime: "15:30"}
	shortClub := PickupExtensionBlock{ID: 3, StartTime: "14:45", EndTime: "15:15", Member: true}
	lateClub := PickupExtensionBlock{ID: 4, StartTime: "15:00", EndTime: "16:30", Member: true}

	assert.Equal(t, []PickupExtensionBlock{freePlay, homework},
		OpenPickupExtensionBlocks([]PickupExtensionBlock{freePlay, homework}, "16:00"),
		"every block the child is not on is a choice")
	assert.Equal(t, []PickupExtensionBlock{freePlay, homework},
		OpenPickupExtensionBlocks([]PickupExtensionBlock{shortClub, freePlay, homework}, "16:00"),
		"a block that ends before the pickup leaves the rest of the time open")
	assert.Nil(t, OpenPickupExtensionBlocks([]PickupExtensionBlock{freePlay, lateClub}, "16:00"),
		"a block running until the pickup looks after the child")
	assert.Empty(t, OpenPickupExtensionBlocks(nil, "16:00"))
}
