package students

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// TestRenderDayLogGroupErrorCodes pins the codes the day log page reads to
// tell a state (no group visible) from a plain refusal (#2517).
func TestRenderDayLogGroupErrorCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"no permitted groups", errNoPermittedDayLogGroups, http.StatusForbidden, common.CodeStudentsDayLogNoGroups},
		{"not group supervisor", errors.New("not_group_supervisor"), http.StatusForbidden, "general.permission"},
		{"dependency failure", errDayLogGroupsUnavailable, http.StatusInternalServerError, "general.server"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/day-log", nil)
			renderDayLogGroupError(rr, req, tc.err, slog.New(slog.DiscardHandler))

			assert.Equal(t, tc.status, rr.Code)
			var body struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body.Code)
		})
	}
}
