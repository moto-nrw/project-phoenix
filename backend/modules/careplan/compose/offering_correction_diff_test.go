package compose

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOfferingCorrectionDiffUsesOnlyFrozenSnapshots(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	changed := testpkg.CreateTestCareOffering(t, db, phase.ID, "Changed")
	unchanged := testpkg.CreateTestCareOffering(t, db, phase.ID, "Unchanged")
	removed := testpkg.CreateTestCareOffering(t, db, phase.ID, "Removed")
	before := json.RawMessage(fmt.Sprintf(`[{"offering_id":"%d","offering_name":"Old name","selected_days":[" FRI ","mon","mon","bad"]},{"offering_id":"%d","offering_name":"Same","selected_days":["tue"]},{"offering_id":"%d","offering_name":"Removed"}]`, changed.ID, unchanged.ID, removed.ID))
	after := json.RawMessage(fmt.Sprintf(`[{"offering_id":"%d","offering_name":"New name","selected_days":["wed"]},{"offering_id":"%d","offering_name":"Same","selected_days":["tue"]}]`, changed.ID, unchanged.ID))
	diff, err := careplan.ReviewCorrectionDiff(before, after)
	require.NoError(t, err)
	require.Len(t, diff, 2)
	require.Equal(t, changed.ID, diff[0].OfferingID)
	require.Equal(t, "New name", diff[0].Label)
	require.Equal(t, []string{"mon", "fri"}, diff[0].OldDays)
	require.Equal(t, []string{"wed"}, diff[0].NewDays)
	require.Equal(t, "removed", diff[1].NewState)
	require.Equal(t, removed.ID, diff[1].OfferingID)
	_, err = careplan.ReviewCorrectionDiff(json.RawMessage(`[{"offering_id":"broken"}]`), nil)
	require.Error(t, err)
	_, err = careplan.ReviewCorrectionDiff(nil, json.RawMessage(`{`))
	require.Error(t, err)
	diff, err = careplan.ReviewCorrectionDiff(nil, nil)
	require.NoError(t, err)
	require.Empty(t, diff)
}
