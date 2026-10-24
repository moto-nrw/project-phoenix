package sessions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSupervisorsResponse_EmptySupervisorSliceSerializesAsJSONArray(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(UpdateSupervisorsResponse{
		ActiveGroupID: 77,
		Supervisors:   []SupervisorInfo{},
		Status:        "success",
		Message:       "Supervisors updated successfully",
	})

	require.NoError(t, err)
	assert.Contains(t, string(payload), `"supervisors":[]`)
}
