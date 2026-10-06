package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Protected activities answer with their own code (#2517), so the web shows
// why the row cannot change instead of the generic permission or conflict
// text. The statuses stay as they were.
func TestActivityErrorRulesNameProtectedActivities(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		renderer   func(error) render.Renderer
		wantStatus int
		wantCode   string
	}{
		{"system activity", systemActivityForbidden, http.StatusForbidden, "timetable.activity_system_protected"},
		{"timetable template", timetableTemplateConflict, http.StatusConflict, "timetable.activity_template_protected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodDelete, "/", nil), tc.renderer(errors.New("delete group: protected"))))
			var body struct {
				Code string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Equal(t, tc.wantCode, body.Code)
		})
	}
}
