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
		"unread":0,
		"audience":"team",
		"readers":{"school":{"kind":"all_staff","names":[]}}
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

// TestThreadViewReadersAndLeadershipOption pins the parent API of #5200: who
// reads the conversation and whether the leadership choice is offered. The
// names of the option appear only while it is available.
func TestThreadViewReadersAndLeadershipOption(t *testing.T) {
	t.Parallel()
	view := toThreadView(&parentService.MessageThreadView{
		StudentID: 82, SchoolName: "Schule am Berg", Audience: parentService.MessageAudienceLeadership,
		GuardianName: "Sabine Schneider", LeadershipReaders: []string{"Swantje M."},
		Leadership: parentService.LeadershipOption{Enabled: true, Names: []string{"Swantje M."}},
	})
	body, err := json.Marshal(struct {
		Audience         string                   `json:"audience"`
		Readers          ParentReadersResponse    `json:"readers"`
		LeadershipOption LeadershipOptionResponse `json:"leadership_option"`
	}{view.Audience, view.Readers, view.LeadershipOption})
	require.NoError(t, err)
	assert.JSONEq(t, `{"audience":"leadership",
		"readers":{"guardian":{"name":"Sabine Schneider"},"school":{"kind":"leadership","names":["Swantje M."]}},
		"leadership_option":{"available":true,"names":["Swantje M."]}}`, string(body))

	off := toThreadView(&parentService.MessageThreadView{
		StudentID: 82, Leadership: parentService.LeadershipOption{Enabled: false, Names: []string{"Swantje M."}},
	})
	assert.Equal(t, parentService.MessageAudienceTeam, off.Audience)
	assert.Equal(t, ParentSchoolReadersResponse{Kind: "all_staff", Names: []string{}}, off.Readers.School)
	assert.Equal(t, LeadershipOptionResponse{Available: false, Names: []string{}}, off.LeadershipOption)

	group := toGroupThreadSummary(parentService.GroupThread{}, "Schule am Berg", nil)
	assert.Equal(t, ParentSchoolReadersResponse{Kind: "all_staff", Names: []string{}}, group.Readers.School, "a group-thread answer goes to the team")
}
