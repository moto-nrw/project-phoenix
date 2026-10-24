package classdayhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fieldErrorBody is one entry of the errors list.
type fieldErrorBody struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// renderedError is the wire body of one error answer.
type renderedError struct {
	Status  int              `json:"-"`
	Code    string           `json:"code"`
	Errors  []fieldErrorBody `json:"errors"`
	Details map[string]any   `json:"details"`
	Detail  string           `json:"detail"`
}

func renderError(t *testing.T, renderer render.Renderer) renderedError {
	t.Helper()
	require.NotNil(t, renderer)
	w := httptest.NewRecorder()
	require.NoError(t, render.Render(w, httptest.NewRequest(http.MethodPost, "/", nil), renderer))
	var body renderedError
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	body.Status = w.Code
	return body
}

func fieldNames(errs []fieldErrorBody) []string {
	names := make([]string, 0, len(errs))
	for _, e := range errs {
		names = append(names, e.Field)
	}
	return names
}

type codeCase struct {
	name       string
	renderer   render.Renderer
	wantStatus int
	wantCode   string
	wantField  string
	wantDetail map[string]any
}

func assertCodeCases(t *testing.T, cases []codeCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := renderError(t, tc.renderer)
			assert.Equal(t, tc.wantStatus, body.Status)
			assert.Equal(t, tc.wantCode, body.Code)
			if tc.wantField == "" {
				assert.Empty(t, body.Errors)
			} else {
				assert.Equal(t, []string{tc.wantField}, fieldNames(body.Errors))
			}
			if tc.wantDetail == nil {
				assert.Empty(t, body.Details)
			} else {
				assert.Equal(t, tc.wantDetail, body.Details)
			}
		})
	}
}

// TestSlotListRefusalCodes pins the codes of the daily lists (#2516).
func TestSlotListRefusalCodes(t *testing.T) {
	t.Parallel()

	assertCodeCases(t, []codeCase{
		{"pickup list in the past", slotListRefusal(classday.ErrPickupCohortPastDate), http.StatusBadRequest, "timetable.slot_list_pickup_past_date", "", nil},
		{"reconciliation in the future", slotListRefusal(classday.ErrReconciliationFutureDate), http.StatusBadRequest, "timetable.slot_list_reconciliation_future_date", "", nil},
		{"drifted", slotListRefusal(fmt.Errorf("render: %w", classday.ErrListDrifted)), http.StatusConflict, "timetable.slot_list_drifted", "", nil},
		{"disabled", slotListRefusal(classday.ErrTimetableDisabled), http.StatusForbidden, "timetable.slot_lists_disabled", "", nil},
	})
	assert.Nil(t, slotListRefusal(errors.New("db down")))
}
