package common_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/require"
)

// Check the assembled generated tables through the public error envelope, not
// only the declarations: losing any shard must change an observable response.
func TestEveryRegisteredErrorCodeKeepsItsProblemClass(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(backendRoot(t), "..", "error-registry.json"))
	require.NoError(t, err)
	var registry struct {
		Codes []struct {
			Code  string `json:"code"`
			Class string `json:"class"`
		} `json:"codes"`
	}
	require.NoError(t, json.Unmarshal(data, &registry))
	require.NotEmpty(t, registry.Codes)
	anchors := map[string]string{
		"input":              "anleitung-eingabe-pruefen",
		"permission":         "anleitung-zugriff-pruefen",
		"business_rejection": "anleitung-vorgang-nicht-moeglich",
		"unavailable":        "anleitung-gerade-nicht-erreichbar",
		"server":             "anleitung-unerwarteter-fehler",
	}
	for _, entry := range registry.Codes {
		// Use a status whose fallback is NOT the registered class, so a missing
		// class mapping cannot accidentally pass through status-based repair.
		recorder := httptest.NewRecorder()
		common.ProblemResponseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if entry.Class == "business_rejection" {
				common.RenderError(w, r, common.ErrorInvalidRequestWithCode(errors.New("class probe"), entry.Code))
				return
			}
			common.RenderError(w, r, common.ErrorConflictWithCode(errors.New("class probe"), entry.Code))
		})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/test", nil))
		var body struct {
			Code string `json:"code"`
			Type string `json:"type"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), entry.Code)
		require.Equal(t, entry.Code, body.Code)
		require.Equal(t, "https://moto-app.de/help/fehlermeldungen#"+anchors[entry.Class], body.Type, entry.Code)
	}
}
