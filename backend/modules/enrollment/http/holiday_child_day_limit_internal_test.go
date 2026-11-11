package enrollmenthttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

func TestPublicHolidayDayLimitWithholdsExistingBookings(t *testing.T) {
	t.Parallel()
	for _, elsewhere := range []int{0, 2, 17} {
		refusal := &capability.HolidayChildDayLimitRefusal{Child: 0, Limit: 3, Days: elsewhere + 4, Elsewhere: elsewhere}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/holiday-registrations/offer", nil)
		renderHolidayRegistrationError(w, r, refusal)
		require.Equal(t, http.StatusBadRequest, w.Code)
		var response struct {
			Code    string         `json:"code"`
			Detail  string         `json:"detail"`
			Details map[string]any `json:"details"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, "enrollment.holiday_child_day_limit_public", response.Code)
		require.Equal(t, capability.ErrHolidayChildDayLimit.Error(), response.Detail)
		require.Equal(t, map[string]any{"limit": float64(3), "child": float64(1)}, response.Details)
	}
}
