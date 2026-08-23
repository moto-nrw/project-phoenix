package slotlists

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSelectedGroupNames_DisclosesGroupWithoutRows guards the #1565 review-pass-2
// fix: result.Groups is derived from the day's rows, so a saved group filter that
// matches no rows for the selected date/source is absent from it. Such an ID must
// still be disclosed in the export header — otherwise an empty or partial
// confidential export prints as if it were unfiltered.
func TestSelectedGroupNames_DisclosesGroupWithoutRows(t *testing.T) {
	t.Parallel()

	t.Run("no selection returns nil", func(t *testing.T) {
		names := selectedGroupNames(Params{}, &Result{
			Groups: []GroupOption{{ID: 1, Name: "Delfine"}},
		})
		assert.Nil(t, names)
	})

	t.Run("resolvable id uses the display name", func(t *testing.T) {
		names := selectedGroupNames(
			Params{GroupIDs: []int64{7}},
			&Result{Groups: []GroupOption{{ID: 7, Name: "Delfine"}}},
		)
		assert.Equal(t, []string{"Delfine"}, names)
	})

	t.Run("id without rows falls back to an explicit marker", func(t *testing.T) {
		// Group 42 has no rows today, so availableOptions never listed it in
		// result.Groups. It must still surface as a restriction.
		names := selectedGroupNames(
			Params{GroupIDs: []int64{42}},
			&Result{Groups: []GroupOption{{ID: 7, Name: "Delfine"}}},
		)
		assert.Equal(t, []string{"Gruppe #42"}, names)
	})

	t.Run("mixed resolvable and unresolvable ids are all disclosed, sorted", func(t *testing.T) {
		names := selectedGroupNames(
			Params{GroupIDs: []int64{7, 42}},
			&Result{Groups: []GroupOption{{ID: 7, Name: "Delfine"}}},
		)
		// Both entries present; sort.Strings keeps output deterministic.
		assert.Equal(t, []string{"Delfine", "Gruppe #42"}, names)
	})
}
