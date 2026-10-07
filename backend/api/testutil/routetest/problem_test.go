package routetest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
)

func TestProblemEnvelopeViolationsNamesEveryDeparture(t *testing.T) {
	t.Parallel()

	shared := `{"status":"error","type":"https://moto-app.de/help/fehlermeldungen#anleitung-eingabe-pruefen","title":"Bad Request","detail":"bad","instance":"req-1","error":"bad","code":"general.input","details":{"id":"1"}}`
	assert.Empty(t, routetest.ProblemEnvelopeViolations([]byte(shared), true))

	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"legacy operator body": {
			body: `{"status":"Too Many Requests","message":"slow down","type":"t","title":"t","detail":"slow down","instance":"req-1","code":"general.unavailable"}`,
			want: []string{`status is "Too Many Requests", want "error"`, `carries "message"; the text belongs in "error"`, "error is missing, want a non-empty string"},
		},
		"bare status text": {
			body: `{"status":"Forbidden"}`,
			want: []string{`status is "Forbidden", want "error"`, "type is missing, want a non-empty string", "title is missing, want a non-empty string", "detail is missing, want a non-empty string", "error is missing, want a non-empty string", "code is missing, want a non-empty string", "instance is missing"},
		},
		"numeric code and empty instance": {
			body: `{"status":"error","type":"t","title":"t","detail":"d","instance":"","error":"d","code":401}`,
			want: []string{"code is 401, want a non-empty string", "instance is empty"},
		},
		"not json": {
			body: "not found",
			want: []string{`not a JSON object: "not found"`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, routetest.ProblemEnvelopeViolations([]byte(tc.body), true))
		})
	}
}
