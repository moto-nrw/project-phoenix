package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// rateLimitBucket is one abuse counter of the public forms: its key type,
// the label its error names, its window and its threshold.
type rateLimitBucket struct {
	keyType     string
	label       string
	window      time.Duration
	maxAttempts int
}

// The thresholds of the online registration; the holiday registration of a
// Träger counts with the same ones. They work for "small school with
// families of three children submitting once" without tuning; a school
// needing others would promote them to settings.
//
// Uploads get their own IP bucket: a family uploads a handful of proofs,
// maybe again after a typo, and the submission bucket would refuse a parent
// who replaces a blurred photo twice.
var (
	ipRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeIP, label: "IP",
		window: time.Hour, maxAttempts: 10,
	}
	emailRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeEmail, label: "email",
		window: rateLimitRetention, maxAttempts: 5,
	}
	uploadRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeUploadIP, label: "upload IP",
		window: time.Hour, maxAttempts: 30,
	}
)

// rateLimitRetention is the longest bucket window. A counter whose window
// started earlier can refuse nothing any more, so the cleanups delete it.
const rateLimitRetention = 24 * time.Hour

// submissionRateLimitRetentionDays reports rateLimitRetention to the
// nightly retention log.
const submissionRateLimitRetentionDays = int(rateLimitRetention / (24 * time.Hour))
