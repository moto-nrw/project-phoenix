package compose

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestOfferingReviewPayloadPreservesStoredSelectionSemantics(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Payload")
	for _, id := range []string{fmt.Sprint(offering.ID), fmt.Sprintf("%q", fmt.Sprint(offering.ID))} {
		selections, err := careplan.ParseOfferingReviewSelections(json.RawMessage(fmt.Sprintf(`{"offerings":[{"offering_id":%s,"selected_days":[" FRI ","mon","mon","unknown"]}]}`, id)))
		require.NoError(t, err)
		require.Len(t, selections, 1)
		require.Equal(t, offering.ID, selections[0].OfferingID)
		require.Equal(t, []string{"mon", "fri"}, selections[0].SelectedDays)
	}
	for _, raw := range []string{`{}`, `{"offerings":null}`, `{"offerings":[null]}`, `{"offerings":[{"offering_id":"bad"}]}`, fmt.Sprintf(`{"offerings":[{"offering_id":%d,"selected_days":[false]}]}`, offering.ID)} {
		_, err := careplan.ParseOfferingReviewSelections(json.RawMessage(raw))
		require.Error(t, err, raw)
	}
	empty, err := careplan.ParseOfferingReviewSelections(json.RawMessage(`{"offerings":[]}`))
	require.NoError(t, err)
	require.NotNil(t, empty, "an explicit complete withdrawal differs from an unreadable payload")
	require.Empty(t, empty)
}

func TestOfferingReviewDecisionDiffPreservesFrozenFacts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phase.ID, "Live name")
	raw := json.RawMessage(fmt.Sprintf(`{"diff":[{"offering_id":%d,"label":"Frozen name","old_state":"not_booked","new_state":"booked","new_days":["fri"],"new_automatic_days":["fri"],"new_rule_days":["fri"],"auto_trigger_names":["Frozen trigger"],"is_course":true}]}`, offering.ID))
	diff, err := careplan.ParseOfferingReviewDecisionDiff(raw)
	require.NoError(t, err)
	require.Len(t, diff, 1)
	require.Equal(t, offering.ID, diff[0].OfferingID)
	require.Equal(t, "Frozen name", diff[0].Label)
	require.Equal(t, []string{"fri"}, diff[0].NewRuleDays)
	require.Equal(t, []string{"Frozen trigger"}, diff[0].AutoTriggerNames)
	require.True(t, diff[0].IsCourse)
	require.Empty(t, diff[0].AutoTriggerIDs, "decision snapshots never stored live trigger IDs")
	_, err = careplan.ParseOfferingReviewDecisionDiff(json.RawMessage(`{"diff":false}`))
	require.Error(t, err)
}
