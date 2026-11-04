package routetest_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
)

func TestProblemEnvelopeViolationsNamesEveryDeparture(t *testing.T) {
	t.Parallel()

	shared := `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Bad Request","status":400,"detail":"bad","instance":"req-1","code":"general.input","details":{"id":"1"}}`
	assert.Empty(t, routetest.ProblemEnvelopeViolations([]byte(shared), http.StatusBadRequest, true))

	for name, tc := range map[string]struct {
		body   string
		status int
		want   []string
	}{
		"legacy envelope": {
			body:   `{"status":"error","error":"bad","type":"t","title":"t","detail":"bad","instance":"req-1","code":"general.input"}`,
			status: http.StatusBadRequest,
			want:   []string{`status is "error", want 400`, `carries "error"; the text belongs in "detail"`},
		},
		"legacy operator body": {
			body:   `{"status":"Too Many Requests","message":"slow down","type":"t","title":"t","detail":"slow down","instance":"req-1","code":"general.unavailable"}`,
			status: http.StatusTooManyRequests,
			want:   []string{`status is "Too Many Requests", want 429`, `carries "message"; the text belongs in "detail"`},
		},
		"status number of another answer": {
			body:   `{"status":404,"type":"t","title":"t","detail":"d","instance":"req-1","code":"general.input"}`,
			status: http.StatusBadRequest,
			want:   []string{"status is 404, want 400"},
		},
		"bare status text": {
			body:   `{"status":"Forbidden"}`,
			status: http.StatusForbidden,
			want:   []string{`status is "Forbidden", want 403`, "type is missing, want a non-empty string", "title is missing, want a non-empty string", "detail is missing, want a non-empty string", "code is missing, want a non-empty string", "instance is missing"},
		},
		"unregistered code": {
			body:   `{"status":409,"type":"t","title":"t","detail":"d","instance":"req-1","code":"care.not_registered"}`,
			status: http.StatusConflict,
			want:   []string{`code "care.not_registered" is not in error-registry/`},
		},
		"numeric code and empty instance": {
			body:   `{"status":401,"type":"t","title":"t","detail":"d","instance":"","code":401}`,
			status: http.StatusUnauthorized,
			want:   []string{"code is 401, want a non-empty string", "instance is empty"},
		},
		"not json": {
			body:   "not found",
			status: http.StatusNotFound,
			want:   []string{`not a JSON object: "not found"`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, routetest.ProblemEnvelopeViolations([]byte(tc.body), tc.status, true))
		})
	}
}
