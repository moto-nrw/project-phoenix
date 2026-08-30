package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Der Frontend-Client ruft die Sammlung ohne Schrägstrich am Ende auf
// (/api/staff-notices), das Backend registriert den Subrouter mit "/".
// chi.Mount bedient beide Schreibweisen mit demselben Handler; dieser Test
// pinnt das über den fertig verdrahteten Serve-Root, damit ein Wechsel des
// Routers oder ein Umbau der Mount-Stelle das nicht stumm zu einem 404 macht.
//
// Deliberately NOT parallel: newGoldenAPI mutates process-global configuration.
func TestStaffNoticesCollectionServedWithoutTrailingSlash(t *testing.T) {
	apiInstance := newGoldenAPI(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/staff-notices"},
		{http.MethodGet, "/api/staff-notices/"},
		{http.MethodPost, "/api/staff-notices"},
		{http.MethodPost, "/api/staff-notices/"},
		{http.MethodGet, "/api/staff-notices/today"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		apiInstance.Router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		// Ohne Token endet die Anfrage in der Auth-Kette (401), nicht im
		// Router-Fallback (404): der Pfad ist also gebunden.
		require.Equalf(t, http.StatusUnauthorized, rec.Code, "%s %s must be routed to the staff-notice resource", tc.method, tc.path)
	}
}
