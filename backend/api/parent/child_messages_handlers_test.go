package parent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestThreadSummaryResponseIncludesStaffReadStatus(t *testing.T) {
	response := toThreadSummary(&usersModels.InboxThread{
		ThreadID:               41,
		StudentID:              82,
		SchoolName:             "Schule am Berg",
		LastMessageReadByStaff: true,
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
		"unread":0
	}`, string(body))
}
