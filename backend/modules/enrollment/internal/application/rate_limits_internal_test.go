package application

import "testing"

// The cleanups delete counters older than rateLimitRetention; a longer
// window would lose its running count and let an abuser start over.
func TestRateLimitRetentionCoversEveryWindow(t *testing.T) {
	t.Parallel()
	for _, bucket := range []rateLimitBucket{ipRateLimit, emailRateLimit, uploadRateLimit} {
		if bucket.window <= 0 || bucket.window > rateLimitRetention {
			t.Errorf("%s window %s outside (0, %s]", bucket.label, bucket.window, rateLimitRetention)
		}
	}
	if submissionRateLimitRetentionDays < 1 {
		t.Errorf("retention log reports %d days", submissionRateLimitRetentionDays)
	}
}
