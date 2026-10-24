package parent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func TestThreadSummaryResponseIncludesStaffReadStatus(t *testing.T) {
	t.Parallel()

	response := toThreadSummary(&parentService.InboxThread{
		ThreadID:               41,
		StudentID:              82,
		SchoolName:             "Schule am Berg",
		LastMessageReadByStaff: true,
		LastMessagePayload:     map[string]json.RawMessage{"effective_from": json.RawMessage(`"2026-09-01"`)},
	})

	body, err := json.Marshal(response)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"thread_id":"41",
		"student_id":"82",
		"student_name":"",
		"school_name":"Schule am Berg",
		"counterpart_name":"OGS Schule am Berg",
		"last_message_read_by_staff":true,
		"last_message_payload":{"effective_from":"2026-09-01"},
		"unread":0
	}`, string(body))
}

func TestNativeMessagePayloadKeepsObjectOmissionAndNumberWireValues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		payload map[string]json.RawMessage
		present bool
	}{
		{name: "nil"},
		{name: "empty", payload: map[string]json.RawMessage{}},
		{name: "number", payload: map[string]json.RawMessage{"large": json.RawMessage(`9007199254740993`), "null": json.RawMessage(`null`)}, present: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := toMessageResponses([]*parentService.ParentMessage{{ID: 17, SenderKind: "guardian", Body: "Message", Payload: test.payload}}, "OGS")
			raw, err := json.Marshal(rows[0])
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &fields))
			payload, ok := fields["payload"]
			assert.Equal(t, test.present, ok)
			if test.present {
				assert.JSONEq(t, `{"large":9007199254740993,"null":null}`, string(payload))
				var values map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(payload, &values))
				assert.Equal(t, json.RawMessage(`9007199254740993`), values["large"])
				assert.Equal(t, json.RawMessage(`null`), values["null"])
			}
		})
	}
}
