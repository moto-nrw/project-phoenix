package httpadapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
)

// Protected activities answer with their own code (#2517), so the web shows
// why the row cannot change instead of the generic permission or conflict
// text. The statuses stay as they were.
func TestActivityErrorRulesNameProtectedActivities(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"system activity", fmt.Errorf("delete group: %w", activitiesSvc.ErrSystemActivityProtected), http.StatusForbidden, common.CodeTimetableActivitySystemProtected},
		{"timetable template", fmt.Errorf("delete group: %w", activitiesSvc.ErrTimetableTemplateProtected), http.StatusConflict, common.CodeTimetableActivityTemplateProtected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodDelete, "/", nil), ErrorRenderer(tc.err)))
			var body struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantCode, body.Code)
		})
	}
}
