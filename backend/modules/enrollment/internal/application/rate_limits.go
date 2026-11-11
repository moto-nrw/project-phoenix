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
// Träger counts with the same ones. A submission counts once it is stored,
// in the transaction that stores it: a refused one (captcha, validation, a
// full week) takes nothing from the budget, and the one over the limit rolls
// back with its count (#6726).
//
// The IP bucket is a backstop behind the captcha and the request limiter. It
// is wide on purpose: the parents of a school share one address in the school
// network, at a parents' evening or behind a mobile carrier. The e-mail
// bucket stays narrow, so nobody floods a foreign address with receipt
// mails. Uploads count per attempt in a bucket of their own, before the body
// is read.
var (
	ipRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeIP, label: "IP",
		window: time.Hour, maxAttempts: 100,
		refusal: enrollment.ErrRateLimited,
	}
	emailRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeEmail, label: "email",
		window: rateLimitRetention, maxAttempts: 10,
		refusal: enrollment.ErrEmailRateLimited,
	}
	uploadRateLimit = rateLimitBucket{
		keyType: enrollment.SubmissionRateLimitKeyTypeUploadIP, label: "upload IP",
		window: time.Hour, maxAttempts: 100,
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
	return errors.Is(err, enrollment.ErrRateLimited) || errors.Is(err, enrollment.ErrEmailRateLimited) ||
		errors.Is(err, enrollment.ErrUploadRateLimited)
}

// RateLimitRefusals reports a refusal to the monitoring under the name of
// its flow and bucket, like the request limiters of the router. Nil reports
// nowhere.
type RateLimitRefusals func(bucket string)

// record reports the refusal of bucket in flow.
func (observe RateLimitRefusals) record(flow string, bucket rateLimitBucket) {
	if observe != nil {
		observe(flow + "-" + strings.ReplaceAll(bucket.keyType, "_", "-"))
	}
}

// countSurvivingRefusal runs count in the unit run opens and keeps the unit
// when count only refuses, so a refused upload still counts. It returns the
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

// countStoredSubmission counts the submission its write transaction just
// stored, per IP address and per e-mail address, and refuses once either
// crosses its threshold; the refusal rolls the submission back with both
// counts. The upsert holds the bucket's row until the transaction ends, so
// parallel submissions queue at the bucket and cannot pass the limit
// together. It is the last lock of the write, IP before e-mail.
//
// A parent with an account does not count against the IP address: the
// session names the submitter, and families in one network must not lock
// each other out. A staff submission counts nowhere.
func (s *Intake) countStoredSubmission(ctx context.Context, req *SubmitRequest) error {
	if req.SkipRateLimit || s.deps.RateLimits == nil || req.TenantID <= 0 {
		return nil
	}
	remoteIP := req.RemoteIP
	if req.GuardianAccountID != nil {
		remoteIP = ""
	}
	for _, key := range submissionAttemptKeys(remoteIP, req.GuardianEmail) {
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
		s.deps.RateLimitRefusals.record("enrollment", bucket)
		s.logger().Info("enrollment submit rate-limited",
			slog.String("key_type", bucket.keyType),
			slog.Int("attempts", state.Attempts),
			slog.Int64("tenant_id", tenantID))
	}
	return refusal
}
