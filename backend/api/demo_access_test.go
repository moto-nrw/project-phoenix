package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoEntryBaseIsTheStandingSchoolsOrigin(t *testing.T) {
	t.Parallel()
	base, err := demoEntryBase("https://demo.moto-app.de", "demo.moto-app.de")
	require.NoError(t, err)
	assert.Equal(t, "https://messe-demo.demo.moto-app.de", base)

	base, err = demoEntryBase("http://localhost:3000", "localhost")
	require.NoError(t, err)
	assert.Equal(t, "http://messe-demo.localhost:3000", base)

	_, err = demoEntryBase("https://demo.moto-app.de", "")
	assert.Error(t, err)
}

// The website calls the demo routes cross-origin and without cookies (#3462).
func TestDemoRoutesAnswerTheWebsitesPreflight(t *testing.T) {
	t.Parallel()
	router := chi.NewRouter()
	router.Use(corsHandler("https://moto-ogs.de,https://www.moto-ogs.de"))
	router.Post("/demo/access-requests", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodOptions, "/demo/access-requests", nil)
	req.Header.Set("Origin", "https://www.moto-ogs.de")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, "https://www.moto-ogs.de", rr.Header().Get("Access-Control-Allow-Origin"))

	req = httptest.NewRequest(http.MethodPost, "/demo/access-requests", strings.NewReader("{}"))
	req.Header.Set("Origin", "https://www.moto-ogs.de")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusAccepted, rr.Code, "a request without cookies passes")
	assert.Equal(t, "https://www.moto-ogs.de", rr.Header().Get("Access-Control-Allow-Origin"))

	req = httptest.NewRequest(http.MethodPost, "/demo/access-requests", strings.NewReader("{}"))
	req.Header.Set("Origin", "https://evil.example")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
}

// A demo environment whose capability was not composed must not start; any
// other environment mounts nothing and needs none.
func TestDemoMountFailsWithoutTheCapability(t *testing.T) {
	t.Parallel()
	assert.Error(t, mountDemoAccess(chi.NewRouter(), nil, " Demo ", "https://demo.example", "demo.example"))
	assert.NoError(t, mountDemoAccess(chi.NewRouter(), nil, "production", "https://demo.example", "demo.example"))
}
