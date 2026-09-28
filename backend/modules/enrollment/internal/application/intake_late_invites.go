package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// Rate-limit thresholds. They work for "small school with families of three
// children submitting once" without tuning; a school needing others would
// promote them to settings.
const (
	rateLimitWindowIP        = time.Hour
	rateLimitWindowEmail     = 24 * time.Hour
	rateLimitMaxAttemptsIP   = 10
	rateLimitMaxAttemptsMail = 5
)

func (s *Intake) newStatusToken() (string, error) {
	if s.deps.Random == nil {
		return "", errors.New("status token entropy source is not configured")
	}
	return enrollment.NewStatusToken(s.deps.Random)
}

// lateInviteTokenHash is the stored identity of a late invite token: the
// fingerprint of the trimmed token.
func (s *Intake) lateInviteTokenHash(token string) string {
	return s.deps.Fingerprint([]byte(strings.TrimSpace(token)))
}

func normalizeGuardianEmail(email string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(email))
	if trimmed == "" {
		return "", fmt.Errorf("%w: guardian email is required", enrollment.ErrInvalidSubmission)
	}
	if err := contact.ValidateOptionalEmail(trimmed); err != nil {
		return "", enrollment.ErrInvalidGuardianEmail
	}
	return trimmed, nil
}

func trimToNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// CreateLateInvite issues a late invite for one family and returns its
// one-time token.
func (s *Intake) CreateLateInvite(ctx context.Context, input enrollment.CreateLateInviteInput) (*enrollment.CreateLateInviteResult, error) {
	if s.deps.LateInvites == nil {
		return nil, fmt.Errorf("late invite repository is not configured")
	}
	if input.PhaseID <= 0 {
		return nil, fmt.Errorf("%w: phase_id is required", enrollment.ErrInvalidSubmission)
	}
	if input.CreatedBy <= 0 {
		return nil, fmt.Errorf("%w: created_by is required", enrollment.ErrInvalidSubmission)
	}
	email, err := normalizeGuardianEmail(input.GuardianEmail)
	if err != nil {
		return nil, err
	}
	phase, err := s.loadPhaseForEditableRequest(ctx, input.PhaseID)
	if err != nil {
		return nil, err
	}
	token, err := s.newStatusToken()
	if err != nil {
		return nil, fmt.Errorf("late invite: generate token: %w", err)
	}
	expiresAt := time.Now().Add(14 * 24 * time.Hour)
	if input.ExpiresAt != nil {
		expiresAt = *input.ExpiresAt
	}
	if !expiresAt.After(time.Now()) {
		return nil, fmt.Errorf("%w: expires_at must be in the future", enrollment.ErrInvalidSubmission)
	}
	invite := &enrollment.LateInvite{
		PhaseID: phase.ID, TokenHash: s.lateInviteTokenHash(token), GuardianEmail: email,
		GuardianFirstName: trimToNil(input.GuardianFirstName), GuardianLastName: trimToNil(input.GuardianLastName),
		ExpiresAt: expiresAt, CreatedBy: input.CreatedBy, Reason: trimToNil(input.Reason),
	}
	if err := s.deps.LateInvites.InsertLateInvite(ctx, invite); err != nil {
		return nil, err
	}
	return &enrollment.CreateLateInviteResult{Invite: invite, Token: token}, nil
}

// findLateInviteForSubmit resolves and locks the invite a submission carries.
// Only a genuinely unusable token is an invalid invite; a lookup failure is a
// server error (#1663).
func (s *Intake) findLateInviteForSubmit(ctx context.Context, token string, phaseID int64, now time.Time) (*enrollment.LateInvite, error) {
	if s.deps.LateInvites == nil {
		return nil, fmt.Errorf("%w: late invite support is not configured", enrollment.ErrLateInviteInvalid)
	}
	invite, err := s.deps.LateInvites.UsableLateInvite(ctx, s.lateInviteTokenHash(token), phaseID, now, true)
	if err != nil {
		if errors.Is(err, enrollment.ErrLateInviteNotFound) {
			return nil, enrollment.ErrLateInviteInvalid
		}
		return nil, fmt.Errorf("submit: resolve late invite: %w", err)
	}
	return invite, nil
}

// CreateManualApprovedEnrollment submits a staff-created enrollment
// (rate-limit skipped, closed window allowed, submission mails suppressed,
// source admin_manual) and approves it in the caller's transaction.
func (s *Intake) CreateManualApprovedEnrollment(ctx context.Context, input enrollment.ManualApprovedEnrollmentInput) (*enrollment.ManualApprovedEnrollmentResult, error) {
	result, err := s.createManualApprovedEnrollment(ctx, input)
	return result, publicError(err)
}

func (s *Intake) createManualApprovedEnrollment(ctx context.Context, input enrollment.ManualApprovedEnrollmentInput) (*enrollment.ManualApprovedEnrollmentResult, error) {
	if s.deps.ManualDecider == nil {
		return nil, errors.New("manual enrollment decider is not configured")
	}
	req, err := decodeSubmitRequest(input.Request)
	if err != nil {
		return nil, err
	}
	req.SkipRateLimit, req.AllowClosedPhase = true, true
	req.SuppressSubmissionEmails, req.ExternalConsentConfirmed = true, true
	req.SubmissionSource = enrollment.RequestSourceAdminManual
	req.SourceMetadata = map[string]any{
		"external_consent_confirmed": true,
		"admin_reason":               input.Reason,
		"send_notification":          input.SendNotification,
		"actor_account_id":           input.ActorID,
	}
	submitted, err := s.submit(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(submitted.Children) != 1 {
		return nil, errors.New("manual approved enrollment produced " + strconv.Itoa(len(submitted.Children)) + " children")
	}
	decided, err := s.deps.ManualDecider.Decide(ctx, enrollment.DecideInput{
		RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID, Status: enrollment.DecisionApproved,
		Reason: input.Reason, ReviewedBy: input.ActorID,
		SuppressParentEmail: !input.SendNotification, SuppressGuardianInvitation: !input.SendNotification,
	})
	if err != nil {
		return nil, err
	}
	request, err := requestInput(submitted.Request)
	if err != nil {
		return nil, err
	}
	return &enrollment.ManualApprovedEnrollmentResult{
		Request: request, Child: decided.Child, StatusURL: submitted.StatusURL, PendingInvite: decided.PendingInvite,
	}, nil
}

// enforceRateLimit counts the attempt in the per-IP and per-email buckets of
// the submission's tenant, in a transaction of its own so the count survives
// a refused submission, and refuses once either crosses its threshold.
// Without a limiter the check is skipped.
func (s *Intake) enforceRateLimit(ctx context.Context, req SubmitRequest) error {
	if s.deps.RateLimits == nil || req.TenantID <= 0 {
		return nil
	}
	if s.deps.Runtime.TenantTx == nil {
		return s.enforceRateLimitBuckets(ctx, req)
	}
	var limitErr error
	rateCtx := s.deps.Runtime.WithoutTransaction(ctx)
	err := s.deps.Runtime.TenantTx(rateCtx, req.TenantID, func(txCtx context.Context) error {
		limitErr = s.enforceRateLimitBuckets(txCtx, req)
		if errors.Is(limitErr, enrollment.ErrRateLimited) {
			return nil
		}
		return limitErr
	})
	if err != nil {
		return fmt.Errorf("enrollment submit: rate-limit transaction: %w", err)
	}
	if errors.Is(limitErr, enrollment.ErrRateLimited) {
		return limitErr
	}
	return nil
}

func (s *Intake) enforceRateLimitBuckets(ctx context.Context, req SubmitRequest) error {
	ip := strings.TrimSpace(req.RemoteIP)
	email := strings.ToLower(strings.TrimSpace(req.GuardianEmail))
	if ip != "" {
		if err := s.countRateLimitAttempt(ctx, req.TenantID, rateLimitBucket{
			keyType: enrollment.SubmissionRateLimitKeyTypeIP, label: "IP", window: rateLimitWindowIP, maxAttempts: rateLimitMaxAttemptsIP,
		}, ip); err != nil {
			return err
		}
	}
	if email != "" {
		return s.countRateLimitAttempt(ctx, req.TenantID, rateLimitBucket{
			keyType: enrollment.SubmissionRateLimitKeyTypeEmail, label: "email", window: rateLimitWindowEmail, maxAttempts: rateLimitMaxAttemptsMail,
		}, email)
	}
	return nil
}

// rateLimitBucket is one submission counter: its key type, the label its
// error names, its window and its threshold.
type rateLimitBucket struct {
	keyType     string
	label       string
	window      time.Duration
	maxAttempts int
}

func (s *Intake) countRateLimitAttempt(ctx context.Context, tenantID int64, bucket rateLimitBucket, keyValue string) error {
	keyType, maxAttempts := bucket.keyType, bucket.maxAttempts
	state, err := s.deps.RateLimits.IncrementAttempts(ctx, tenantID, keyType, keyValue, bucket.window)
	if err != nil {
		return fmt.Errorf("enrollment submit: rate-limit %s increment: %w", bucket.label, err)
	}
	if state.Attempts > maxAttempts {
		s.logger().Info("enrollment submit rate-limited",
			slog.String("key_type", keyType),
			slog.Int("attempts", state.Attempts),
			slog.Int64("tenant_id", tenantID))
		return enrollment.ErrRateLimited
	}
	return nil
}

// cloneSourceMetadata copies a metadata map so the submission can add to it.
func cloneSourceMetadata(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
