package staffmessaging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func TestStaffMessagingGroupLimitErrorsIncludeStructuredLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		code    string
		details map[string]int
	}{
		{
			name:    "name",
			err:     communication.ErrStaffMessageGroupNameInvalid,
			code:    "communication.staff_group_name_invalid",
			details: map[string]int{"max_characters": communication.MaxStaffGroupNameRunes},
		},
		{
			name:    "members",
			err:     communication.ErrStaffMessageGroupTooLarge,
			code:    "communication.staff_group_too_large",
			details: map[string]int{"max_members": communication.MaxStaffGroupMembers},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			renderStaffMessagingError(rec, httptest.NewRequest(http.MethodPost, "/", nil), tt.err)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			var body struct {
				Code    string         `json:"code"`
				Details map[string]int `json:"details"`
			}
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			require.Equal(t, tt.code, body.Code)
			require.Equal(t, tt.details, body.Details)
		})
	}
}
