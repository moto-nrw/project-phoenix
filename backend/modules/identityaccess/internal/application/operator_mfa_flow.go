package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// OperatorMFAFlows is the operator's second factor (#3331): the e-mail
// challenge, its verification, the enrollment and its cascade, and the
// remember-device cookies. The cooldown, the send cap and the code lifetime
// are fixed for operators by design — every moto operator faces the same
// posture, and the per-school security.* settings apply to accounts, which
// operators are not.
//
// The flows run over the module's own operator and operator-MFA records.
// They join an administrative transaction when the caller opened one and
// otherwise run on the root connection, except for the disable cascade,
// which opens its own so the three writes commit as one unit.
type OperatorMFAFlows struct {
	operators *Service
	records   *OperatorMFA
	codec     ports.MFAChallengeCodec
	codes     ports.ShortCodeHasher
	mail      ports.MFAMail
	audit     ports.MFAAuditTrail
	runtime   ports.Runtime
	secret    []byte
	logger    *slog.Logger
	now       func() time.Time
}

// OperatorMFAFlowDependencies are the ports the operator MFA flows consume.
type OperatorMFAFlowDependencies struct {
	Records *OperatorMFA
	Codec   ports.MFAChallengeCodec
	Codes   ports.ShortCodeHasher
	Mail    ports.MFAMail
	Audit   ports.MFAAuditTrail
	Runtime ports.Runtime
	// Secret is the trusted-device HMAC key derived from the JWT secret.
	Secret []byte
	Logger *slog.Logger
}

// NewOperatorMFAFlows composes the flows over the operator service.
func NewOperatorMFAFlows(operators *Service, deps OperatorMFAFlowDependencies) (*OperatorMFAFlows, error) {
	switch {
	case operators == nil, deps.Records == nil, deps.Codec == nil, deps.Codes == nil,
		deps.Mail == nil, deps.Audit == nil, deps.Runtime == nil, len(deps.Secret) == 0:
		return nil, errors.New("identity access operator mfa: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &OperatorMFAFlows{
		operators: operators, records: deps.Records, codec: deps.Codec, codes: deps.Codes,
		mail: deps.Mail, audit: deps.Audit, runtime: deps.Runtime, secret: deps.Secret,
		logger: logger.With("component", "operator-mfa"), now: time.Now,
	}, nil
}

// ===== Inquiry =====

// HasEnrollment reports whether the operator enrolled in e-mail MFA. A
// missing enrollment is (false, nil) — every fresh operator hits it on the
// first login. Any other failure refuses this login rather than fail-open.
func (f *OperatorMFAFlows) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	credential, found, err := f.findCredential(ctx, operatorID)
	if err != nil {
		f.logger.Warn("operator mfa enrollment lookup failed; refusing login",
			slog.Int64("operator_id", operatorID),
			slog.String("error", err.Error()))
		return false, domain.ErrMFAStatusUnavailable
	}
	return found && credential.ID > 0, nil
}

// TrustedDeviceDays is the fixed operator remember-device lifetime in days.
func (f *OperatorMFAFlows) TrustedDeviceDays() int {
	return int(domain.OperatorMFATrustedDeviceDuration.Hours() / 24)
}

// ===== Challenge and verification =====

// StartChallenge mails a fresh code and returns the challenge JWT it named.
// The row is stored consumed and only activated once the transport accepted
// the message, so a failed send never leaves a redeemable code behind.
func (f *OperatorMFAFlows) StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	operator, err := f.operators.findOperatorByID(ctx, operatorID)
	if err != nil {
		return "", fmt.Errorf("look up operator: %w", err)
	}
	if domain.MFALocked(operator.MFALockedUntil, f.now()) {
		return "", domain.ErrMFALocked
	}

	if err := f.requireChallengeRateLimit(ctx, operatorID); err != nil {
		return "", err
	}
	plainCode, err := domain.GenerateEmailCode()
	if err != nil {
		return "", fmt.Errorf("generate email code: %w", err)
	}
	codeHash, err := f.codes.HashShortCode(plainCode)
	if err != nil {
		return "", fmt.Errorf("hash email code: %w", err)
	}

	createdAt := f.now()
	challenge, send, err := f.enqueueChallenge(ctx, domain.OperatorMFAChallenge{
		OperatorID: operatorID, CodeHash: codeHash, ExpiresAt: createdAt.Add(domain.OperatorMFAChallengeTTL),
		ConsumedAt: &createdAt, IPAddress: ip,
	}, domain.MFACodeMailFor(
		operator.Email, operator.DisplayName, operator.ID, true, plainCode,
		domain.OperatorMFAChallengeTTL, ip, true, f.TrustedDeviceDays(),
	))
	if err != nil {
		return "", fmt.Errorf("prepare operator email challenge: %w", err)
	}
	if err := send(f.runtime.Detach(ctx)); err != nil {
		f.logger.Warn("operator mfa challenge delivery failed",
			slog.Int64("operator_id", operator.ID),
			slog.String("error", err.Error()),
		)
		return "", domain.ErrMFAStatusUnavailable
	}
	if err := f.activateDeliveredChallenge(ctx, challenge); err != nil {
		f.logger.Error("failed to activate delivered operator mfa challenge",
			slog.Int64("challenge_id", challenge.ID),
			slog.String("error", err.Error()),
		)
		return "", domain.ErrMFAStatusUnavailable
	}

	// The operator id reuses the account slot; the scope distinguishes the
	// two id spaces.
	token, err := f.codec.IssueChallengeToken(domain.MFAChallengeClaims{
		AccountID: operatorID,
		Scope:     domain.MFAChallengeScopePlatform,
	}, domain.OperatorMFAChallengeTTL)
	if err != nil {
		return "", fmt.Errorf("mint operator challenge jwt: %w", err)
	}
	return token, nil
}

// VerifyChallenge redeems a code against the challenge JWT that carried it.
func (f *OperatorMFAFlows) VerifyChallenge(ctx context.Context, challengeToken, code string) (domain.OperatorVerifiedChallenge, error) {
	claims, err := f.codec.ParseChallengeToken(challengeToken)
	if err != nil || claims.Scope != domain.MFAChallengeScopePlatform {
		return domain.OperatorVerifiedChallenge{}, domain.ErrMFAChallengeTokenInvalid
	}
	challenge, err := f.verifyActiveCode(ctx, claims.AccountID, code, domain.ErrMFAChallengeTokenInvalid)
	if err != nil {
		return domain.OperatorVerifiedChallenge{}, err
	}
	credential, found, credErr := f.findCredential(ctx, claims.AccountID)
	if credErr == nil && found && credential.ID > 0 {
		_ = f.records.TouchCredential(ctx, credential.ID, challenge.consumedAt)
	}
	return domain.OperatorVerifiedChallenge{OperatorID: claims.AccountID}, nil
}

// ResendChallenge re-issues a code for the challenge the token names and
// returns the renewed JWT, so the frontend can replace the in-flight token.
func (f *OperatorMFAFlows) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	claims, err := f.codec.ParseChallengeToken(challengeToken)
	if err != nil || claims.Scope != domain.MFAChallengeScopePlatform {
		return "", domain.ErrMFAChallengeTokenInvalid
	}
	return f.StartChallenge(ctx, claims.AccountID, ip)
}

// VerifyCodeForOperator is the JWT-less sibling of VerifyChallenge: the
// enrollment confirmation and the passkey registration have already
// authenticated the operator out of band.
func (f *OperatorMFAFlows) VerifyCodeForOperator(ctx context.Context, operatorID int64, code string) error {
	_, err := f.verifyActiveCode(ctx, operatorID, code, domain.ErrMFACodeInvalid)
	return err
}

// redeemed names the row a verification consumed and when.
type redeemed struct {
	id         int64
	consumedAt time.Time
}

// verifyActiveCode resolves the operator's active code, compares it and
// consumes it exactly once. The loser of two concurrent verifications is
// refused rather than allowed to mint a second session from one code.
func (f *OperatorMFAFlows) verifyActiveCode(ctx context.Context, operatorID int64, code string, lookupError error) (redeemed, error) {
	var result redeemed
	var verificationErr error
	err := f.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		operator, err := f.operators.findOperatorByID(txCtx, operatorID)
		if err != nil {
			return lookupError
		}
		if domain.MFALocked(operator.MFALockedUntil, f.now()) {
			return domain.ErrMFALocked
		}
		result, verificationErr = f.verifyActiveCodeInTx(txCtx, operator, code)
		// Invalid codes still commit their failure counters and audit evidence.
		if verificationErr != nil && !errors.Is(verificationErr, domain.ErrMFACodeInvalid) {
			return verificationErr
		}
		return nil
	})
	if err != nil {
		return redeemed{}, err
	}
	return result, verificationErr
}

func (f *OperatorMFAFlows) verifyActiveCodeInTx(ctx context.Context, operator domain.Operator, code string) (redeemed, error) {
	active, err := f.records.FindActiveChallenge(ctx, operator.ID)
	if err != nil {
		if err := f.recordAudit(ctx, operator.ID, domain.OperatorAuditActionMFAFailed, nil, nil, &domain.OperatorMFAEvidence{Reason: "no active challenge"}); err != nil {
			return redeemed{}, err
		}
		return redeemed{}, domain.ErrMFACodeInvalid
	}
	ok, verifyErr := f.codes.VerifyShortCode(code, active.CodeHash)
	if verifyErr != nil || !ok {
		if err := f.handleFailedAttempt(ctx, operator); err != nil {
			return redeemed{}, err
		}
		if err := f.recordAudit(ctx, operator.ID, domain.OperatorAuditActionMFAFailed, nil, &active.ID, &domain.OperatorMFAEvidence{Reason: "code mismatch"}); err != nil {
			return redeemed{}, err
		}
		return redeemed{}, domain.ErrMFACodeInvalid
	}
	now := f.now()
	if err := f.records.ConsumeChallenge(ctx, active.ID, now); err != nil {
		f.logger.Warn("failed to mark operator challenge consumed; refusing verify",
			slog.Int64("operator_id", operator.ID),
			slog.Int64("challenge_id", active.ID),
			slog.String("error", err.Error()))
		if err := f.recordAudit(ctx, operator.ID, domain.OperatorAuditActionMFAFailed, nil, &active.ID, &domain.OperatorMFAEvidence{Reason: "consume race"}); err != nil {
			return redeemed{}, err
		}
		return redeemed{}, domain.ErrMFACodeInvalid
	}
	// One UPDATE, so a concurrent failed verify's increment cannot be
	// overwritten by a stale full-row write.
	if err := f.operators.ResetOperatorMFAAttempts(ctx, operator.ID); err != nil {
		f.logger.Warn("failed to reset operator MFA attempts",
			slog.String("error", err.Error()),
		)
	}
	if err := f.recordAudit(ctx, operator.ID, domain.OperatorAuditActionMFAVerified, nil, &active.ID, nil); err != nil {
		return redeemed{}, err
	}
	return redeemed{id: active.ID, consumedAt: now}, nil
}

// handleFailedAttempt bumps the lockout counter atomically and records the
// cooldown exactly when this increment crossed the threshold.
func (f *OperatorMFAFlows) handleFailedAttempt(ctx context.Context, operator domain.Operator) error {
	result, err := f.operators.IncrementOperatorMFAAttempts(ctx, operator.ID, domain.MFALockoutThreshold, domain.MFALockoutDuration)
	if err != nil {
		f.logger.Warn("failed to persist operator MFA attempt counter",
			slog.String("error", err.Error()),
		)
		return err
	}
	if result.Attempts == domain.MFALockoutThreshold {
		return f.recordAudit(ctx, operator.ID, domain.OperatorAuditActionMFALocked, nil, nil, &domain.OperatorMFAEvidence{LockedUntil: result.LockedUntil})
	}
	return nil
}

// ===== Enrollment =====

// Enroll records the operator's e-mail enrollment once.
func (f *OperatorMFAFlows) Enroll(ctx context.Context, operatorID int64) error {
	return f.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		existing, found, _ := f.findCredential(txCtx, operatorID)
		if found && existing.ID > 0 {
			return domain.ErrMFAAlreadyEnrolled
		}
		if _, err := f.records.CreateCredential(txCtx, domain.OperatorMFACredential{
			OperatorID: operatorID,
			Method:     domain.MFAMethodEmail,
			EnrolledAt: f.now(),
		}); err != nil {
			return fmt.Errorf("persist operator mfa credential: %w", err)
		}
		return f.recordAudit(txCtx, operatorID, domain.OperatorAuditActionMFAEnrolled, nil, nil, nil)
	})
}

// Disable wipes the enrollment, revokes every trusted device and resets the
// lockout counter in one transaction, so a partial failure cannot leave the
// operator half-disabled: a credential gone while remember-device cookies
// still verify, or a counter stuck at the threshold.
func (f *OperatorMFAFlows) Disable(ctx context.Context, operatorID int64) error {
	err := f.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if err := f.records.DeleteCredentials(txCtx, operatorID); err != nil {
			return fmt.Errorf("delete operator credential: %w", err)
		}
		if err := f.records.RevokeTrustedDevices(txCtx, operatorID, f.now()); err != nil {
			return fmt.Errorf("revoke operator trusted devices: %w", err)
		}
		if err := f.operators.ResetOperatorMFAAttempts(txCtx, operatorID); err != nil {
			return fmt.Errorf("reset operator mfa attempts: %w", err)
		}
		return f.recordAudit(txCtx, operatorID, domain.OperatorAuditActionMFADisabled, nil, nil, nil)
	})
	if err != nil {
		return err
	}
	return nil
}

// ===== Trusted devices =====

// IssueTrustedDevice mints a remember-device cookie and notifies the
// operator by mail that a device was added.
func (f *OperatorMFAFlows) IssueTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	expiresAt := f.now().Add(domain.OperatorMFATrustedDeviceDuration)
	rawToken, err := domain.GenerateTrustedDeviceToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate operator trusted-device token: %w", err)
	}
	device := domain.OperatorTrustedDevice{
		OperatorID: operatorID,
		TokenHash:  domain.HashTrustedDeviceToken(rawToken),
		IPAddress:  ip,
		ExpiresAt:  expiresAt,
	}
	if userAgent != "" {
		label := userAgent
		device.UserAgent = &label
	}
	err = f.runtime.WithAdminTx(f.runtime.Detach(ctx), func(txCtx context.Context) error {
		stored, err := f.records.CreateTrustedDevice(txCtx, device)
		if err != nil {
			return fmt.Errorf("persist operator trusted device: %w", err)
		}
		if err := f.recordAudit(txCtx, operatorID, domain.OperatorAuditActionMFATrustedDeviceAdded, ip, &stored.ID, nil); err != nil {
			return err
		}
		return f.notifyTrustedDeviceAdded(txCtx, operatorID, stored.ID, userAgent, ip)
	})
	if err != nil {
		return "", time.Time{}, err
	}
	signed := domain.SignTrustedDeviceToken(rawToken, f.secret)
	return signed, expiresAt, nil
}

// VerifyTrustedDevice reports whether the cookie names an active device of
// this operator, and touches it when it does.
func (f *OperatorMFAFlows) VerifyTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error) {
	rawToken, ok := domain.VerifyTrustedDeviceToken(signedCookie, f.secret)
	if !ok {
		return false, nil
	}
	device, err := f.records.FindActiveTrustedDevice(ctx, operatorID, domain.HashTrustedDeviceToken(rawToken))
	if err != nil {
		return false, nil
	}
	_ = f.records.TouchTrustedDevice(ctx, device.ID, f.now())
	return true, nil
}

// RevokeTrustedDevice revokes one device after proving it belongs to the
// calling operator, so an id guess cannot revoke someone else's.
func (f *OperatorMFAFlows) RevokeTrustedDevice(ctx context.Context, operatorID, deviceID int64) error {
	devices, err := f.records.ListActiveTrustedDevices(ctx, operatorID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if device.ID == deviceID {
			return f.records.RevokeTrustedDevice(ctx, deviceID, f.now())
		}
	}
	return domain.ErrMFAPermissionDenied
}

// ===== Helpers =====

func (f *OperatorMFAFlows) findCredential(ctx context.Context, operatorID int64) (domain.OperatorMFACredential, bool, error) {
	credential, err := f.records.FindCredential(ctx, operatorID)
	if errors.Is(err, identityaccess.ErrOperatorMFACredentialNotFound) {
		return domain.OperatorMFACredential{}, false, nil
	}
	if err != nil {
		return domain.OperatorMFACredential{}, false, err
	}
	return credential, true, nil
}

func (f *OperatorMFAFlows) notifyTrustedDeviceAdded(ctx context.Context, operatorID, deviceID int64, userAgent string, ip net.IP) error {
	operator, err := f.operators.findOperatorByID(ctx, operatorID)
	if err != nil {
		f.logger.Warn("could not load operator for trusted-device-added mail",
			slog.Int64("operator_id", operatorID),
			slog.String("error", err.Error()))
		return errors.New("could not load operator for trusted-device-added mail")
	}
	return f.mail.NotifyTrustedDeviceAdded(ctx, domain.TrustedDeviceMail{
		Recipient: operator.Email, RecipientName: operator.DisplayName, ReferenceID: operatorID, Operator: true,
		DeviceLabel: domain.ShortenUserAgent(userAgent), RequestIP: domain.AuditIPString(ip),
		AddedAt: f.now().Format("02.01.2006 15:04"), TrustedDays: f.TrustedDeviceDays(),
	}, deviceID)
}

func (f *OperatorMFAFlows) recordAudit(ctx context.Context, operatorID int64, action string, ip net.IP, resourceID *int64, evidence *domain.OperatorMFAEvidence) error {
	return f.audit.RecordOperatorAction(ctx, domain.OperatorAuditEntry{
		OperatorID: operatorID, Action: action, ResourceType: domain.OperatorAuditResourceOperatorMFA,
		ResourceID: resourceID, IPAddress: domain.AuditIPString(ip), MFA: evidence,
	})
}

func (f *OperatorMFAFlows) enqueueChallenge(ctx context.Context, input domain.OperatorMFAChallenge, message domain.MFACodeMail) (domain.OperatorMFAChallenge, func(context.Context) error, error) {
	var challenge domain.OperatorMFAChallenge
	var send func(context.Context) error
	err := f.runtime.WithAdminTx(f.runtime.Detach(ctx), func(txCtx context.Context) error {
		var err error
		challenge, err = f.records.CreateChallenge(txCtx, input)
		if err != nil {
			return fmt.Errorf("persist email challenge: %w", err)
		}
		send, err = f.mail.EnqueueCode(txCtx, message, challenge.ID, challenge.ExpiresAt)
		return err
	})
	return challenge, send, err
}

func (f *OperatorMFAFlows) activateDeliveredChallenge(ctx context.Context, challenge domain.OperatorMFAChallenge) error {
	return f.runtime.WithAdminTx(f.runtime.Detach(ctx), func(txCtx context.Context) error {
		if err := f.records.ActivateChallenge(txCtx, challenge.ID); err != nil {
			return err
		}
		return f.recordAudit(txCtx, challenge.OperatorID, domain.OperatorAuditActionMFAEmailSent, challenge.IPAddress, &challenge.ID, nil)
	})
}

func (f *OperatorMFAFlows) requireChallengeRateLimit(ctx context.Context, operatorID int64) error {
	// The hard cap of three codes per fifteen minutes is an abuse defense,
	// so a failed count refuses the send instead of waving it through:
	// ignoring the error meant the cap silently stopped existing exactly
	// when the database was least healthy.
	since := f.now().Add(-domain.OperatorMFARateLimitWindow)
	count, err := f.records.CountChallengesSince(ctx, operatorID, since)
	if err != nil {
		f.logger.Warn("operator mfa rate-limit lookup failed; refusing to issue a code",
			slog.Int64("operator_id", operatorID),
			slog.String("error", err.Error()))
		return domain.ErrMFAStatusUnavailable
	}
	if count >= domain.OperatorMFARateLimitMaxSent {
		return domain.ErrMFARateLimited
	}

	return nil
}

func (f *OperatorMFAFlows) HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	if f == nil {
		return false, identityaccess.ErrOperatorMFAUnavailable
	}
	enrolled, err := f.HasEnrollment(ctx, operatorID)
	return enrolled, operatorFlowError(err)
}

func (f *OperatorMFAFlows) OperatorTrustedDeviceDays() int {
	if f == nil {
		return 0
	}
	return f.TrustedDeviceDays()
}

func (f *OperatorMFAFlows) StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	if f == nil {
		return "", identityaccess.ErrOperatorMFAUnavailable
	}
	token, err := f.StartChallenge(ctx, operatorID, ip)
	return token, operatorFlowError(err)
}

// VerifyOperatorMFAChallenge returns the operator the redeemed code belongs
// to, so the login mints its token pair without re-reading the JWT.
func (f *OperatorMFAFlows) VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (int64, error) {
	if f == nil {
		return 0, identityaccess.ErrOperatorMFAUnavailable
	}
	verified, err := f.VerifyChallenge(ctx, challengeToken, code)
	if err != nil {
		return 0, operatorFlowError(err)
	}
	return verified.OperatorID, nil
}

func (f *OperatorMFAFlows) ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if f == nil {
		return "", identityaccess.ErrOperatorMFAUnavailable
	}
	token, err := f.ResendChallenge(ctx, challengeToken, ip)
	return token, operatorFlowError(err)
}

func (f *OperatorMFAFlows) VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string) error {
	if f == nil {
		return identityaccess.ErrOperatorMFAUnavailable
	}
	return operatorFlowError(f.VerifyCodeForOperator(ctx, operatorID, code))
}

func (f *OperatorMFAFlows) EnrollOperatorMFA(ctx context.Context, operatorID int64) error {
	if f == nil {
		return identityaccess.ErrOperatorMFAUnavailable
	}
	return operatorFlowError(f.Enroll(ctx, operatorID))
}

func (f *OperatorMFAFlows) DisableOperatorMFA(ctx context.Context, operatorID int64) error {
	if f == nil {
		return identityaccess.ErrOperatorMFAUnavailable
	}
	return operatorFlowError(f.Disable(ctx, operatorID))
}

func (f *OperatorMFAFlows) IssueOperatorTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	if f == nil {
		return "", time.Time{}, identityaccess.ErrOperatorMFAUnavailable
	}
	cookie, expiresAt, err := f.IssueTrustedDevice(ctx, operatorID, userAgent, ip)
	return cookie, expiresAt, operatorFlowError(err)
}

func (f *OperatorMFAFlows) VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error) {
	if f == nil {
		return false, identityaccess.ErrOperatorMFAUnavailable
	}
	verified, err := f.VerifyTrustedDevice(ctx, operatorID, signedCookie)
	return verified, operatorFlowError(err)
}

func (f *OperatorMFAFlows) ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]identityaccess.OperatorTrustedDevice, error) {
	if f == nil {
		return nil, identityaccess.ErrOperatorMFAUnavailable
	}
	devices, err := f.records.ListActiveTrustedDevices(ctx, operatorID)
	if err != nil {
		return nil, operatorFlowError(err)
	}
	result := make([]identityaccess.OperatorTrustedDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, identityaccess.OperatorTrustedDevice(device))
	}
	return result, nil
}

func (f *OperatorMFAFlows) RevokeOperatorTrustedDeviceOwned(ctx context.Context, operatorID, deviceID int64) error {
	if f == nil {
		return identityaccess.ErrOperatorMFAUnavailable
	}
	return operatorFlowError(f.RevokeTrustedDevice(ctx, operatorID, deviceID))
}
