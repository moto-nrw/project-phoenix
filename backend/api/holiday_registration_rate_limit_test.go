package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The public links (the holiday link, the school's online enrollment) are
// exempt from the general limiter, so their own limit must never be tighter
// than the general one. Production runs the general limiter at 600 a minute
// with a burst of 100; the first limit of the holiday link, 300 a minute,
// halved what the link had before (#6726).
func TestPublicLinkLimitsAreNeverTighterThanTheGeneralLimiter(t *testing.T) {
	t.Parallel()
	for _, general := range []struct{ limit, burst int }{
		{60, 10},     // defaults of the code
		{600, 100},   // production and staging
		{5000, 1000}, // a general limit above the floor of the link
	} {
		limit, burst := publicLinkLimits(general.limit, general.burst)
		assert.GreaterOrEqual(t, limit, general.limit, "requests per minute beside a general limit of %d", general.limit)
		assert.GreaterOrEqual(t, burst, general.burst, "burst beside a general burst of %d", general.burst)
		assert.GreaterOrEqual(t, limit, publicLinkRequestsPerMinute)
		assert.GreaterOrEqual(t, burst, publicLinkBurst)
	}
}

// A school network opens the link at once: every request of the burst
// passes, the one after it is refused, reads and writes apart.
func TestHolidayRegistrationRateLimitLetsItsBurstThrough(t *testing.T) {
	t.Parallel()
	limited := holidayRegistrationRateLimit(true, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	send := func(method string) int {
		rec := httptest.NewRecorder()
		limited.ServeHTTP(rec, httptest.NewRequest(method, holidayRegistrationPath+"/token", nil))
		return rec.Code
	}
	_, burst := publicLinkLimits(generalRateLimitFromEnv())
	for i := range burst {
		if code := send(http.MethodGet); code != http.StatusOK {
			t.Fatalf("read %d of the burst of %d: status %d", i+1, burst, code)
		}
	}
	assert.Equal(t, http.StatusTooManyRequests, send(http.MethodGet), "the read past the burst")
	assert.Equal(t, http.StatusOK, send(http.MethodPost), "writes have a budget of their own")
	assert.Nil(t, holidayRegistrationRateLimit(false, false), "no limiter while rate limiting is off")
}
