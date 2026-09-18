package workforce_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Browser clients send BIGINT identifiers as decimal strings (#3258); the
// admin handlers decode these public request types directly.
func TestAbsenceRequestsAcceptDecimalStringIDs(t *testing.T) {
	t.Parallel()

	t.Run("create", func(t *testing.T) {
		t.Parallel()
		var req workforce.CreateAbsenceRequest
		require.NoError(t, json.Unmarshal([]byte(`{"absence_type":"other","absence_type_id":"9007199254740993","date_start":"2026-08-07","date_end":"2026-08-07"}`), &req))
		require.NotNil(t, req.AbsenceTypeID)
		assert.Equal(t, int64(9007199254740993), *req.AbsenceTypeID)
		assert.Equal(t, "2026-08-07", req.DateStart)
	})

	t.Run("update keeps null apart from omitted", func(t *testing.T) {
		t.Parallel()
		var cleared workforce.UpdateAbsenceRequest
		require.NoError(t, json.Unmarshal([]byte(`{"absence_type":"training","absence_type_id":null}`), &cleared))
		assert.True(t, cleared.AbsenceTypeIDSet)
		assert.Nil(t, cleared.AbsenceTypeID)

		var omitted workforce.UpdateAbsenceRequest
		require.NoError(t, json.Unmarshal([]byte(`{"note":"x"}`), &omitted))
		assert.False(t, omitted.AbsenceTypeIDSet)
		require.NotNil(t, omitted.Note)

		var named workforce.UpdateAbsenceRequest
		require.NoError(t, json.Unmarshal([]byte(`{"absence_type_id":"12"}`), &named))
		assert.True(t, named.AbsenceTypeIDSet)
		require.NotNil(t, named.AbsenceTypeID)
		assert.Equal(t, int64(12), *named.AbsenceTypeID)
	})

	t.Run("rebook", func(t *testing.T) {
		t.Parallel()
		var req workforce.RebookAbsencesRequest
		require.NoError(t, json.Unmarshal([]byte(`{"absence_ids":["7",8],"absence_type":"other","absence_type_id":"9007199254740995","reason":"Kontingent angelegt","dry_run":true}`), &req))
		assert.Equal(t, []int64{7, 8}, req.AbsenceIDs)
		require.NotNil(t, req.AbsenceTypeID)
		assert.Equal(t, int64(9007199254740995), *req.AbsenceTypeID)
		assert.Equal(t, "Kontingent angelegt", req.Reason)
		assert.True(t, req.DryRun)
	})

	t.Run("rebook rejects malformed ids", func(t *testing.T) {
		t.Parallel()
		var req workforce.RebookAbsencesRequest
		require.Error(t, json.Unmarshal([]byte(`{"absence_ids":["x"]}`), &req))
		require.Error(t, json.Unmarshal([]byte(`{"absence_ids":[null]}`), &req))
		require.Error(t, json.Unmarshal([]byte(`{"absence_type_id":"1.5"}`), &req))
	})
}
