package slotlists

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestApplyGrouping_SlotDisambiguatesDistinctInstances guards the #1565
// review-pass-2 fix: GroupBySlot keys on the Slot label, but that label is not
// unique across concrete activity instances (parallel offerings in different
// rooms, or two templates with the same title/time). Without disambiguation the
// export's repeated-marker suppression and the frontend's group_title bucketing
// fold distinct instances into one section. Each distinct instance must therefore
// receive its own GroupTitle.
func TestApplyGrouping_SlotDisambiguatesDistinctInstances(t *testing.T) {
	t.Parallel()

	t.Run("unique slot labels stay clean", func(t *testing.T) {
		rows := []Row{
			{InstanceID: 1, Slot: "Fußball 14:00–15:00", StudentID: 10},
			{InstanceID: 2, Slot: "Basteln 14:00–15:00", StudentID: 20},
		}
		applyGrouping(rows, GroupBySlot)
		assert.Equal(t, "Angebot: Fußball 14:00–15:00", rows[0].GroupTitle)
		assert.Equal(t, "Angebot: Basteln 14:00–15:00", rows[1].GroupTitle)
	})

	t.Run("same label different rooms disambiguates by room", func(t *testing.T) {
		rows := []Row{
			{InstanceID: 1, Slot: "Fußball 14:00–15:00", RoomName: "Sporthalle", StudentID: 10},
			{InstanceID: 2, Slot: "Fußball 14:00–15:00", RoomName: "Kunstraum", StudentID: 20},
		}
		applyGrouping(rows, GroupBySlot)
		assert.Equal(t, "Angebot: Fußball 14:00–15:00 (Sporthalle)", rows[0].GroupTitle)
		assert.Equal(t, "Angebot: Fußball 14:00–15:00 (Kunstraum)", rows[1].GroupTitle)
		assert.NotEqual(t, rows[0].GroupTitle, rows[1].GroupTitle, "distinct instances must never share a section")
	})

	t.Run("same label same room falls back to a stable ordinal", func(t *testing.T) {
		rows := []Row{
			{InstanceID: 2, Slot: "Fußball 14:00–15:00", RoomName: "Sporthalle", StudentID: 20},
			{InstanceID: 1, Slot: "Fußball 14:00–15:00", RoomName: "Sporthalle", StudentID: 10},
		}
		applyGrouping(rows, GroupBySlot)
		// Ordinals follow ascending instance ID (deterministic), regardless of row
		// input order: instance 1 → (1), instance 2 → (2).
		byInstance := map[int64]string{}
		for _, r := range rows {
			byInstance[r.InstanceID] = r.GroupTitle
		}
		assert.Equal(t, "Angebot: Fußball 14:00–15:00 (1)", byInstance[1])
		assert.Equal(t, "Angebot: Fußball 14:00–15:00 (2)", byInstance[2])
	})

	t.Run("mixed collision uses room where unique and ordinal otherwise", func(t *testing.T) {
		rows := []Row{
			{InstanceID: 1, Slot: "AG 14:00", RoomName: "Raum A", StudentID: 10},
			{InstanceID: 2, Slot: "AG 14:00", RoomName: "", StudentID: 20},
			{InstanceID: 3, Slot: "AG 14:00", RoomName: "", StudentID: 30},
		}
		applyGrouping(rows, GroupBySlot)
		byInstance := map[int64]string{}
		for _, r := range rows {
			byInstance[r.InstanceID] = r.GroupTitle
		}
		assert.Equal(t, "Angebot: AG 14:00 (Raum A)", byInstance[1])
		assert.Equal(t, "Angebot: AG 14:00 (1)", byInstance[2])
		assert.Equal(t, "Angebot: AG 14:00 (2)", byInstance[3])
		// All three headings are distinct.
		assert.Len(t, map[string]struct{}{
			byInstance[1]: {}, byInstance[2]: {}, byInstance[3]: {},
		}, 3)
	})

	t.Run("numeric room name never collides with an ordinal", func(t *testing.T) {
		// A unique room literally named "1" renders the suffix " (1)", which is
		// textually identical to the first ordinal " (1)" the shared-room / no-room
		// instances fall back to. The two namespaces must stay disjoint so distinct
		// instances never share a GroupTitle (#1565 review pass 2 follow-up).
		rows := []Row{
			{InstanceID: 1, Slot: "AG 14:00", RoomName: "1", StudentID: 10},
			{InstanceID: 2, Slot: "AG 14:00", RoomName: "", StudentID: 20},
			{InstanceID: 3, Slot: "AG 14:00", RoomName: "", StudentID: 30},
		}
		applyGrouping(rows, GroupBySlot)
		byInstance := map[int64]string{}
		for _, r := range rows {
			byInstance[r.InstanceID] = r.GroupTitle
		}
		// The unique room keeps its name; the ordinal pass skips the " (1)" string
		// the room already claimed and starts at " (2)".
		assert.Equal(t, "Angebot: AG 14:00 (1)", byInstance[1])
		assert.Equal(t, "Angebot: AG 14:00 (2)", byInstance[2])
		assert.Equal(t, "Angebot: AG 14:00 (3)", byInstance[3])
		assert.Len(t, map[string]struct{}{
			byInstance[1]: {}, byInstance[2]: {}, byInstance[3]: {},
		}, 3, "distinct instances must never share a section")
	})

	t.Run("non-slot grouping is unaffected", func(t *testing.T) {
		rows := []Row{
			{InstanceID: 1, Slot: "Fußball", SchoolClass: "3a", StudentID: 10},
			{InstanceID: 2, Slot: "Fußball", SchoolClass: "3b", StudentID: 20},
		}
		applyGrouping(rows, GroupByClass)
		assert.Equal(t, "Klasse: 3a", rows[0].GroupTitle)
		assert.Equal(t, "Klasse: 3b", rows[1].GroupTitle)
	})
}
