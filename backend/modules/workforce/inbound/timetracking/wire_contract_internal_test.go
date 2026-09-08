package timetracking

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

func TestHistoryWirePreservesLargeIDsAndCalculatedFields(t *testing.T) {
	t.Parallel()
	const largeID int64 = 9007199254740993
	response := workforce.HistoryResponse{Sessions: []*workforce.SessionResponse{{
		WorkSession:  &workforce.WorkSession{ID: largeID, TenantID: largeID, StaffID: largeID, CreatedBy: largeID, UpdatedBy: new(largeID), BreakMinutes: 10},
		BreakMinutes: 30, NetMinutes: 450, EditCount: 2, AuditCount: 3,
	}}}
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	var decoded struct {
		Sessions []map[string]json.RawMessage `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Sessions, 1)
	row := decoded.Sessions[0]
	for _, field := range []string{"id", "tenant_id", "staff_id", "created_by", "updated_by"} {
		require.JSONEq(t, `"9007199254740993"`, string(row[field]), field)
	}
	require.JSONEq(t, "30", string(row["break_minutes"]), "calculated breaks override the cache")
	require.JSONEq(t, "450", string(row["net_minutes"]))
	require.JSONEq(t, "2", string(row["edit_count"]))
	require.JSONEq(t, "3", string(row["audit_count"]))
	response.Sessions[0].UpdatedBy = nil
	raw, err = json.Marshal(response.Sessions[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "updated_by")
	raw, err = json.Marshal(workforce.SessionResponse{NetMinutes: 7})
	require.NoError(t, err)
	require.Contains(t, string(raw), `"net_minutes":7`)
	require.NotContains(t, string(raw), `"id"`)
}

func TestScheduleWireUsesMinutePrecision(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"08:30:00", "08:30:45"} {
		t.Run(input, func(t *testing.T) {
			entries, _, _, err := scheduleRowsToResponseParts([]workforce.StaffWorkSchedule{{StartTime: input}})
			require.NoError(t, err)
			require.Equal(t, new("08:30"), entries[0].StartTime)
			templates, _, err := modelEntriesToResponseParts([]workforce.WorkTimeModelEntry{{StartTime: input}}, 1)
			require.NoError(t, err)
			require.Equal(t, new("08:30"), templates[0].StartTime)
		})
	}
	entries, _, _, err := scheduleRowsToResponseParts([]workforce.StaffWorkSchedule{{}})
	require.NoError(t, err)
	require.Nil(t, entries[0].StartTime)
	_, _, _, err = scheduleRowsToResponseParts([]workforce.StaffWorkSchedule{{StartTime: "invalid"}})
	require.ErrorContains(t, err, "stored start time")
	_, _, err = modelEntriesToResponseParts([]workforce.WorkTimeModelEntry{{StartTime: "invalid"}}, 1)
	require.ErrorContains(t, err, "stored start time")
}
