package timetablehttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type pickupFailureState string

func (s pickupFailureState) Error() string { return "ERROR: " + string(s) }
func (s pickupFailureState) Field(field byte) string {
	if field == 'C' {
		return string(s)
	}
	return ""
}

// Each failure point of a pickup extension resolution answers with its
// status and code (#5816): the refusals keep their 4xx codes, every database
// failure is a 500.
func TestPickupExtensionResolveFailureStatuses(t *testing.T) {
	t.Parallel()
	lockFailure := func(state string) error {
		return fmt.Errorf("timetable postgres: find activity instance: %w", pickupFailureState(state))
	}
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "task missing", err: timetable.ErrPickupExtensionNotFound, status: http.StatusNotFound, code: "timetable.pickup_extension_not_found"},
		{name: "block gone", err: timetable.ErrPickupExtensionBlockGone, status: http.StatusConflict, code: "timetable.pickup_extension_block_gone"},
		{name: "invalid input", err: timetable.ErrInvalidPickupExtension, status: http.StatusBadRequest, code: "timetable.pickup_extension_invalid"},
		{name: "lock timeout", err: lockFailure("55P03"), status: http.StatusInternalServerError},
		{name: "cancelled wait", err: lockFailure("57014"), status: http.StatusInternalServerError},
		{name: "deadlock", err: lockFailure("40P01"), status: http.StatusInternalServerError},
		{name: "trigger failure", err: fmt.Errorf("timetable postgres: ensure instance student: %w", pickupFailureState("P0001")), status: http.StatusInternalServerError},
		{name: "enrollment gone", err: timetable.ErrStudentEnrollmentNotFound, status: http.StatusInternalServerError},
		{name: "unknown commit outcome", err: errors.New("unit of work: commit outcome unknown"), status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			renderPickupExtensionError(w, httptest.NewRequest(http.MethodPost, "/pickup-extensions/1/resolve", nil), tc.err)
			assert.Equal(t, tc.status, w.Code)
			if tc.code != "" {
				var body struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.Equal(t, tc.code, body.Code)
			}
		})
	}
}
