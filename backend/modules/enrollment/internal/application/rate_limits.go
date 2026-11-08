package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// rateLimitBucket is one abuse counter of the public forms: its key type,
// the label its error names, its window, its threshold and the error it
// refuses with.
type rateLimitBucket struct {
	keyType     string
	label       string
	window      time.Duration
	maxAttempts int
	refusal     error
}

// refuse is the bucket's refusal once attempts cross its threshold.
func (b rateLimitBucket) refuse(attempts int) error {
	if attempts > b.maxAttempts {
		return b.refusal
	}
	return nil
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
		refusal: enrollment.ErrRateLimited,
	}
	emailRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeEmail, label: "email",
		window: rateLimitRetention, maxAttempts: 5,
		refusal: enrollment.ErrRateLimited,
	}
	uploadRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeUploadIP, label: "upload IP",
		window: time.Hour, maxAttempts: 30,
		refusal: enrollment.ErrUploadRateLimited,
	}
)

// rateLimitRetention is the longest bucket window. A counter whose window
// started earlier can refuse nothing any more, so the cleanups delete it.
const rateLimitRetention = 24 * time.Hour

// submissionRateLimitRetentionDays reports rateLimitRetention to the
// nightly retention log.
const submissionRateLimitRetentionDays = int(rateLimitRetention / (24 * time.Hour))

// rateLimitKey is one bucket and the value an attempt counts under.
type rateLimitKey struct {
	bucket rateLimitBucket
	value  string
}

// submissionAttemptKeys are the buckets a submission counts in: the IP
// address and the lower-cased e-mail address, each skipped when empty.
func submissionAttemptKeys(remoteIP, email string) []rateLimitKey {
	keys := make([]rateLimitKey, 0, 2)
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		keys = append(keys, rateLimitKey{ipRateLimit, ip})
	}
	if email := guardianEmailKey(email); email != "" {
		keys = append(keys, rateLimitKey{emailRateLimit, email})
	}
	return keys
}

// isRateLimitRefusal reports whether err is a bucket's refusal.
func isRateLimitRefusal(err error) bool {
	return errors.Is(err, enrollment.ErrRateLimited) || errors.Is(err, enrollment.ErrUploadRateLimited)
}

// countSurvivingRefusal runs count in the unit run opens and keeps the unit
// when count only refuses, so a refused attempt still counts. It returns the
// unit's failure or the refusal.
func countSurvivingRefusal(run func(func(context.Context) error) error, count func(context.Context) error) error {
	var refusal error
	if err := run(func(txCtx context.Context) error {
		err := count(txCtx)
		if isRateLimitRefusal(err) {
			refusal = err
			return nil
		}
		return err
	}); err != nil {
		return err
	}
	return refusal
}

// countSubmissionAttempt counts the submission in the per-IP and per-email
// buckets and refuses once either crosses its threshold.
func (s *Intake) countSubmissionAttempt(ctx context.Context, req SubmitRequest) error {
	for _, key := range submissionAttemptKeys(req.RemoteIP, req.GuardianEmail) {
		if err := s.countRateLimitAttempt(ctx, req.TenantID, key.bucket, key.value); err != nil {
			return err
		}
	}
	return nil
}

func (s *Intake) countRateLimitAttempt(ctx context.Context, tenantID int64, bucket rateLimitBucket, keyValue string) error {
	state, err := s.deps.RateLimits.IncrementAttempts(ctx, tenantID, bucket.keyType, keyValue, bucket.window)
	if err != nil {
		return fmt.Errorf("enrollment submit: rate-limit %s increment: %w", bucket.label, err)
	}
	refusal := bucket.refuse(state.Attempts)
	if refusal != nil {
		s.logger().Info("enrollment submit rate-limited",
			slog.String("key_type", bucket.keyType),
			slog.Int("attempts", state.Attempts),
			slog.Int64("tenant_id", tenantID))
	}
	return refusal
}
