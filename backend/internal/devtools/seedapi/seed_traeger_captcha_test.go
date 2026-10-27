package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The demo server holds a real Turnstile key, which refuses the seeder's
// test token. The demo school order must still succeed: the holiday period
// stays published without registrations, and the confirmation letters step
// skips the run it never got.
func TestSeedTraegerHolidayPeriodStepSkipsTheRegistrationsWhenTurnstileRefusesTheTestToken(t *testing.T) {
	t.Parallel()
	for _, refusal := range []struct {
		status int
		body   string
	}{
		{http.StatusBadRequest, `{"status":400,"code":"enrollment.captcha_failed","detail":"captcha verification failed"}`},
		// Every demo order counts against the one demo Träger and seeded address.
		{http.StatusTooManyRequests, `{"status":429,"code":"enrollment.submission_rate_limited","detail":"too many submission attempts"}`},
	} {
		t.Run(fmt.Sprint(refusal.status), func(t *testing.T) {
			t.Parallel()
			assertSeedHolidayRefusalSkipsTheRegistrations(t, refusal.status, refusal.body)
		})
	}
}

func assertSeedHolidayRefusalSkipsTheRegistrations(t *testing.T, status int, body string) {
	t.Helper()
	var calls []string
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
		case "/traeger/schools/":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":31},{"id":32}]}`))
		case "/traeger/holiday-periods/":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		case "/traeger/holiday-periods/5/publish":
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"status":"published","public_token":"link-key","weeks":[` +
				`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63},{"id":64}]}]}}`))
		case "/traeger/holiday-registrations/link-key":
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		default:
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":9}}`))
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	require.NoError(t, (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime))
	assert.Equal(t, []string{
		"POST /traeger/auth/login",
		"GET /traeger/schools/",
		"POST /traeger/holiday-periods/",
		"POST /traeger/holiday-periods/5/publish",
		"POST /traeger/contribution-rules/",
		"POST /traeger/contribution-rules/",
		"POST /traeger/holiday-quotes/link-key",
		"POST /traeger/holiday-registrations/link-key",
	}, calls, "the step stops after the refused registration and bills no run")

	calls = nil
	require.NoError(t, (seedTraegerBillingLettersStep{}).Run(context.Background(), runtime))
	assert.Empty(t, calls, "without registrations there is no run to write letters for")
}

func TestSeedTraegerHolidayPeriodStepStillFailsOnOtherRegistrationErrors(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/traeger/auth/login":
			_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"traeger-token","refresh_token":"r"}`))
		case "/traeger/schools/":
			_, _ = w.Write([]byte(`{"status":"success","data":[{"id":31}]}`))
		case "/traeger/holiday-periods/5/publish":
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5,"status":"published","public_token":"link-key","weeks":[` +
				`{"id":51,"time_models":[{"id":61},{"id":62}]},{"id":52,"time_models":[{"id":63},{"id":64}]}]}}`))
		case "/traeger/holiday-registrations/link-key":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":400,"code":"enrollment.holiday_registration_invalid","detail":"the guardian e-mail is missing"}`))
		default:
			_, _ = w.Write([]byte(`{"status":"success","data":{"id":5}}`))
		}
	})
	defer srv.Close()

	runtime := &Runtime{
		Client:    newTestClient(srv.URL, false),
		Bootstrap: &bootstrapSeedState{SchoolID: 31, AdminEmail: "admin@example.test", AdminPassword: testpkg.NewTestPassword()},
	}
	err := (seedTraegerHolidayPeriodStep{}).Run(context.Background(), runtime)
	require.ErrorContains(t, err, "register for the holiday period")
	_, skipped := runtime.Values[seedHolidayRegistrationsSkippedKey]
	assert.False(t, skipped)
}
