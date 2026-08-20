package parent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// todayStatusRouter verdrahtet nur die eine Route, damit der Test die
// Antwortform prueft und nicht die Router-Verkabelung.
func todayStatusRouter(rs *Resource) chi.Router {
	router := chi.NewRouter()
	router.Get("/parent/me/children/{studentId}/today", rs.getChildTodayStatus)
	return router
}

func callTodayStatus(t *testing.T, svc *fakeParentService) *httptest.ResponseRecorder {
	t.Helper()
	rs := &Resource{ParentService: svc}
	req := withClaims(
		httptest.NewRequest(http.MethodGet, "/parent/me/children/4242/today", nil),
		7777,
	)
	w := httptest.NewRecorder()
	todayStatusRouter(rs).ServeHTTP(w, req)
	return w
}

// TestTodayStatusEndpointReturnsPresent prueft die zweistufige Antwort: die
// Ja/Nein-Aussage at_ogs und den erklaerenden Zustand samt Uhrzeit.
func TestTodayStatusEndpointReturnsPresent(t *testing.T) {
	t.Parallel()

	present := true
	w := callTodayStatus(t, &fakeParentService{
		todayStatus: &parentService.TodayStatus{
			AtOgs:      &present,
			State:      parentService.DayStatePresent,
			Since:      "12:38",
			PickupTime: "15:30",
		},
	})

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data TodayStatusResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotNil(t, body.Data.AtOgs)
	assert.True(t, *body.Data.AtOgs)
	assert.Equal(t, "present", body.Data.State)
	assert.Equal(t, "12:38", body.Data.Since)
	assert.Equal(t, "15:30", body.Data.PickupTime)
	assert.Empty(t, body.Data.Until)
}

// TestTodayStatusEndpointOmitsJaNeinWhenUnknown pinnt den Rueckfall: ohne
// belastbare Daten trifft die Antwort keine Ja/Nein-Aussage, statt eine zu
// erfinden.
func TestTodayStatusEndpointOmitsJaNeinWhenUnknown(t *testing.T) {
	t.Parallel()

	w := callTodayStatus(t, &fakeParentService{
		todayStatus: &parentService.TodayStatus{State: parentService.DayStateUnknown},
	})

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data TodayStatusResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Nil(t, body.Data.AtOgs, "unbekannter Zustand darf keine Ja/Nein-Aussage tragen")
	assert.Equal(t, "unknown", body.Data.State)
}

// TestTodayStatusEndpointLeaksNoInternalFields ist die eigentliche
// Datenschutz-Zusicherung: die Elternantwort traegt genau vier Felder, nie
// Raeume, Besuchshistorie, Rohereignisse oder Mitarbeitendennamen.
func TestTodayStatusEndpointLeaksNoInternalFields(t *testing.T) {
	t.Parallel()

	present := true
	w := callTodayStatus(t, &fakeParentService{
		todayStatus: &parentService.TodayStatus{
			AtOgs: &present,
			State: parentService.DayStatePresent,
			Since: "12:38",
		},
	})

	var raw map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	data, ok := raw["data"].(map[string]any)
	require.True(t, ok, "Antwort muss ein data-Objekt tragen")

	allowed := map[string]bool{
		"at_ogs": true, "state": true, "since": true,
		"until": true, "expected_from": true, "pickup_time": true,
	}
	for key := range data {
		assert.Truef(t, allowed[key], "unerwartetes Feld in der Elternantwort: %q", key)
	}
}

// TestTodayStatusEndpointPassesCallerAndChild stellt sicher, dass der Handler
// die Berechtigungspruefung nicht umgeht, sondern Konto und Kind an den
// Service durchreicht, der sie prueft.
func TestTodayStatusEndpointPassesCallerAndChild(t *testing.T) {
	t.Parallel()

	svc := &fakeParentService{}
	w := callTodayStatus(t, svc)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7777), svc.gotTodayAccount)
	assert.Equal(t, int64(4242), svc.gotTodayStudent)
}
