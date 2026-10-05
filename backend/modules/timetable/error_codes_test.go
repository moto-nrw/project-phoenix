package timetable

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodedErrorKeepsTextAndChain pins that attaching a wire code changes
// neither the diagnostic text nor the sentinel chain (#2516).
func TestCodedErrorKeepsTextAndChain(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("%w: effective_date must not be in the past", ErrSplitInvalidInput)
	err := WithCode(inner, CodeTemplateSplitInPast)
	assert.Equal(t, inner.Error(), err.Error())
	require.ErrorIs(t, err, ErrSplitInvalidInput)
	coded, ok := AsCoded(fmt.Errorf("wrapped: %w", err))
	require.True(t, ok)
	assert.Equal(t, CodeTemplateSplitInPast, coded.ErrorCode())
	assert.Nil(t, coded.ErrorDetails())
	assert.Equal(t, RefusalValues{MaxDays: 56}, WithCode(errors.New("x"), CodeWindowTooLarge, RefusalValues{MaxDays: 56}).(*CodedError).ErrorDetails())
}

// TestPlanningTrackDraftNamesItsField pins the field each invalid draft
// marks in the editor.
func TestPlanningTrackDraftNamesItsField(t *testing.T) {
	t.Parallel()

	cases := map[string]PlanningTrackDraft{
		"name":       {Color: "#112233"},
		"color":      {Name: "Jahrgang 1", Color: "rot"},
		"sort_order": {Name: "Jahrgang 1", Color: "#112233", SortOrder: -1},
	}
	for field, draft := range cases {
		_, err := draft.Validate()
		var invalid *InvalidPlanningTrackError
		require.ErrorAs(t, err, &invalid)
		assert.Equal(t, field, invalid.Field)
		require.ErrorIs(t, err, ErrInvalidPlanningTrack)
	}
}
