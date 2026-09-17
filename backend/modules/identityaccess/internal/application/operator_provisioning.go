package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// OperatorProvisioning runs the operator invitation and e-mail change flows
// (#3332): inviting a new operator, the public validation and acceptance of
// that link, resend, revoke and cleanup, and the verified change of an
// operator's own address.
//
// Operators are platform-wide rows, so every multi-statement flow opens an
// administrative transaction. The order inside it is the security contract:
// the inviter's row is locked first so two parallel invitations cannot both
// pass the rate limit, the limit is checked before any existing link is
// spent so a refused request never destroys the invitee's last usable link,
// and the acceptance spends the token before it hashes a password so a bogus
// token cannot buy Argon2id work.
type OperatorProvisioning struct {
	operators *Service
	tokens    *OperatorTokens
	passwords ports.PasswordVerifier
	hasher    ports.PasswordHasher
	format    ports.EmailFormat
	audit     ports.OperatorAudit
	invites   ports.OperatorInvitationDelivery
	changes   ports.OperatorEmailChangeDelivery
	runtime   ports.Runtime
	expiry    OperatorProvisioningExpiry
	now       func() time.Time
	logger    *slog.Logger
}

// OperatorProvisioningExpiry is how long each link stays redeemable.
type OperatorProvisioningExpiry struct {
	Invitation  time.Duration
	EmailChange time.Duration
}

// OperatorProvisioningDependencies are the ports the flows consume.
type OperatorProvisioningDependencies struct {
	Tokens    *OperatorTokens
	Passwords ports.PasswordVerifier
	Hasher    ports.PasswordHasher
	Format    ports.EmailFormat
	Audit     ports.OperatorAudit
	Invites   ports.OperatorInvitationDelivery
	Changes   ports.OperatorEmailChangeDelivery
	Runtime   ports.Runtime
	Expiry    OperatorProvisioningExpiry
	Logger    *slog.Logger
}

func NewOperatorProvisioning(operators *Service, deps OperatorProvisioningDependencies) (*OperatorProvisioning, error) {
	switch {
	case operators == nil, deps.Tokens == nil:
		return nil, fmt.Errorf("identity access operator provisioning: the operator service and the token flows are required")
	case deps.Passwords == nil, deps.Hasher == nil, deps.Format == nil:
		return nil, fmt.Errorf("identity access operator provisioning: password verifier, password hasher and address format are required")
	case deps.Audit == nil, deps.Invites == nil, deps.Changes == nil, deps.Runtime == nil:
		return nil, fmt.Errorf("identity access operator provisioning: audit, both deliveries and the tenant runtime are required")
	case deps.Expiry.Invitation <= 0, deps.Expiry.EmailChange <= 0:
		return nil, fmt.Errorf("identity access operator provisioning: positive link expiries are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &OperatorProvisioning{
		operators: operators, tokens: deps.Tokens, passwords: deps.Passwords, hasher: deps.Hasher,
		format: deps.Format, audit: deps.Audit, invites: deps.Invites, changes: deps.Changes,
		runtime: deps.Runtime, expiry: deps.Expiry, now: time.Now,
		logger: logger.With("component", "operator-provisioning"),
	}, nil
}

// --- invitation ------------------------------------------------------------

// InviteOperator creates the link that lets the address become an operator
// and mails it.
func (p *OperatorProvisioning) InviteOperator(ctx context.Context, request domain.OperatorInvitationRequest) error {
	email, err := domain.NormalizeOperatorEmail(request.Email, p.format.IsRoutable)
	if err != nil {
		return err
	}

	// An address that already is an operator is reported: an invitation is
	// an administrative act, not something an unauthenticated caller does,
	// so there is nothing to enumerate here.
	if _, err := p.operators.FindOperatorByEmail(ctx, email); err == nil {
		return domain.ErrOperatorEmailExists
	} else if !errors.Is(err, domain.ErrOperatorNotFound) {
		return fmt.Errorf("check operator email uniqueness: %w", err)
	}

	var invitation domain.OperatorInvitation
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		// Lock the inviter's row so two parallel invitations from the same
		// operator serialize: under READ COMMITTED both would otherwise read
		// the same count and both pass the limit.
		if _, lockErr := p.operators.FindOperatorForUpdate(txCtx, request.CreatedBy); lockErr != nil {
			return fmt.Errorf("lock inviter row: %w", lockErr)
		}
		recent, countErr := p.tokens.CountInvitationsCreatedAfter(txCtx, request.CreatedBy, p.now().Add(-domain.OperatorInvitationRateWindow))
		if countErr != nil {
			return fmt.Errorf("check invitation rate limit: %w", countErr)
		}
		if recent >= domain.OperatorInvitationRateLimit {
			return domain.ErrOperatorInvitationRateLimited
		}
		if _, revokeErr := p.tokens.RevokeInvitationsForEmail(txCtx, email); revokeErr != nil {
			return fmt.Errorf("revoke previous invitations: %w", revokeErr)
		}
		created, createErr := p.tokens.CreateInvitation(txCtx, domain.OperatorInvitation{
			Email:       email,
			Token:       uuid.Must(uuid.NewV4()).String(),
			ExpiresAt:   p.now().Add(p.expiry.Invitation),
			CreatedBy:   request.CreatedBy,
			DisplayName: request.DisplayName,
		})
		if createErr != nil {
			return createErr
		}
		invitation = created
		return nil
	}); err != nil {
		return err
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: request.CreatedBy, Action: domain.OperatorAuditActionInvitationCreated,
		ResourceType: domain.OperatorAuditResourceInvitation, ResourceID: &invitation.ID, IPAddress: request.IPAddress,
	}, "operator invitation")

	p.invites.DispatchOperatorInvitation(ctx, invitation, p.inviterName(ctx, request.CreatedBy))
	return nil
}

// ValidateOperatorInvitation returns what the public accept page may show
// about a still redeemable link.
func (p *OperatorProvisioning) ValidateOperatorInvitation(ctx context.Context, token string) (domain.OperatorInvitationPreview, error) {
	invitation, err := p.tokens.FindRedeemableInvitation(ctx, token)
	if err != nil {
		return domain.OperatorInvitationPreview{}, err
	}
	return domain.OperatorInvitationPreview{
		Email: invitation.Email, DisplayName: invitation.DisplayName, ExpiresAt: invitation.ExpiresAt,
	}, nil
}

// AcceptOperatorInvitation creates the operator the link promises.
func (p *OperatorProvisioning) AcceptOperatorInvitation(ctx context.Context, acceptance domain.OperatorInvitationAcceptance) (domain.Operator, error) {
	// Refuse the cheap mistakes before any database work.
	if err := p.hasher.ValidatePasswordStrength(acceptance.Password); err != nil {
		return domain.Operator{}, domain.InvalidInput(domain.MessageOperatorPasswordTooWeak)
	}
	displayName, err := domain.NormalizeOperatorDisplayName(acceptance.DisplayName)
	if err != nil {
		return domain.Operator{}, err
	}

	var operator domain.Operator
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		// Spend the token first: it is the cheap statement, and a bogus or
		// expired link must not buy an Argon2id hash. A later failure rolls
		// the whole transaction back, so the link stays usable.
		invitation, redeemErr := p.tokens.RedeemInvitation(txCtx, acceptance.Token)
		if redeemErr != nil {
			return redeemErr
		}
		if _, findErr := p.operators.FindOperatorByEmail(txCtx, invitation.Email); findErr == nil {
			return domain.ErrOperatorEmailExists
		} else if !errors.Is(findErr, domain.ErrOperatorNotFound) {
			return fmt.Errorf("check operator email uniqueness: %w", findErr)
		}
		hash, hashErr := p.hasher.HashPassword(acceptance.Password)
		if hashErr != nil {
			return fmt.Errorf("hash operator password: %w", hashErr)
		}
		created, createErr := p.operators.CreateOperator(txCtx, domain.Operator{
			Email: invitation.Email, DisplayName: displayName, PasswordHash: hash, Active: true,
		})
		if createErr != nil {
			return createErr
		}
		operator = created
		return nil
	}); err != nil {
		return domain.Operator{}, err
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: operator.ID, Action: domain.OperatorAuditActionInvitationAccepted,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &operator.ID, IPAddress: acceptance.IPAddress,
	}, "operator invitation acceptance")

	return operator, nil
}

// ListPendingOperatorInvitations returns the links that can still be spent.
func (p *OperatorProvisioning) ListPendingOperatorInvitations(ctx context.Context) ([]domain.OperatorInvitation, error) {
	return p.tokens.ListRedeemableInvitations(ctx)
}

// RevokeOperatorInvitation spends a redeemable link without redeeming it.
func (p *OperatorProvisioning) RevokeOperatorInvitation(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if err := p.tokens.RevokeInvitation(ctx, invitationID); err != nil {
		return err
	}
	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: actorID, Action: domain.OperatorAuditActionInvitationRevoked,
		ResourceType: domain.OperatorAuditResourceInvitation, ResourceID: &invitationID, IPAddress: ipAddress,
	}, "invitation revocation")
	return nil
}

// ResendOperatorInvitation extends a still redeemable link and mails it
// again. An expired or spent link is never revived.
func (p *OperatorProvisioning) ResendOperatorInvitation(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if err := p.tokens.ExtendInvitation(ctx, invitationID, p.now().Add(p.expiry.Invitation)); err != nil {
		return err
	}
	// The delivery tracking starts over, so the mail the invitee gets now is
	// the one the screens report on.
	if err := p.tokens.RecordInvitationDelivery(ctx, invitationID, domain.TokenDelivery{}); err != nil {
		return fmt.Errorf("reset invitation delivery tracking: %w", err)
	}
	invitation, err := p.tokens.FindInvitation(ctx, invitationID)
	if err != nil {
		return err
	}

	p.invites.DispatchOperatorInvitation(ctx, invitation, p.inviterName(ctx, actorID))

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: actorID, Action: domain.OperatorAuditActionInvitationResent,
		ResourceType: domain.OperatorAuditResourceInvitation, ResourceID: &invitationID, IPAddress: ipAddress,
	}, "invitation resend")
	return nil
}

// DeleteExpiredOperatorInvitations removes the links nobody can spend.
func (p *OperatorProvisioning) DeleteExpiredOperatorInvitations(ctx context.Context) (int, error) {
	return p.tokens.DeleteExpiredInvitations(ctx)
}

// inviterName reads the display name the invitation mail shows. A failure is
// not worth refusing the invitation over: the mail falls back to the generic
// wording the template carries.
func (p *OperatorProvisioning) inviterName(ctx context.Context, inviterID int64) string {
	inviter, err := p.operators.FindOperator(ctx, inviterID)
	if err != nil {
		return ""
	}
	return inviter.DisplayName
}

// --- e-mail change ---------------------------------------------------------

// InitiateOperatorEmailChange verifies the operator's current password and
// mails a confirmation link to the requested address.
//
// A requested address that already belongs to another operator is answered
// with success and no mail: the caller is authenticated, but reporting it
// would turn this endpoint into an operator directory. The rate limit runs
// before that check so the observable behavior does not depend on whether
// the address exists.
func (p *OperatorProvisioning) InitiateOperatorEmailChange(ctx context.Context, request domain.OperatorEmailChangeRequest) error {
	operator, err := p.operators.FindOperator(ctx, request.OperatorID)
	if err != nil {
		return err
	}
	if !operator.Active {
		return domain.ErrOperatorInactive
	}
	// Argon2id is expensive, so it runs before the transaction is opened.
	match, verifyErr := p.passwords.VerifyPassword(request.CurrentPassword, operator.PasswordHash)
	if verifyErr != nil || !match {
		return domain.ErrOperatorPasswordMismatch
	}
	// The hash the check accepted, so a password rotation that commits in
	// the meantime is detected inside the transaction without a second hash.
	provenHash := operator.PasswordHash

	newEmail, err := domain.NormalizeOperatorEmail(request.NewEmail, p.format.IsRoutable)
	if err != nil {
		return err
	}
	if newEmail == operator.Email {
		return domain.ErrOperatorEmailChangeSameEmail
	}
	maskedNewEmail := domain.MaskOperatorEmail(newEmail)

	var change domain.OperatorEmailChange
	var addressTaken bool
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		// Lock the operator's row so two parallel requests serialize instead
		// of both relying on the partial unique index to catch the conflict.
		if _, lockErr := p.operators.FindOperatorForUpdate(txCtx, request.OperatorID); lockErr != nil {
			return fmt.Errorf("lock operator row: %w", lockErr)
		}
		recent, countErr := p.tokens.CountEmailChangesCreatedAfter(txCtx, request.OperatorID, p.now().Add(-domain.OperatorEmailChangeRateWindow))
		if countErr != nil {
			return fmt.Errorf("check email change rate limit: %w", countErr)
		}
		if recent >= domain.OperatorEmailChangeRateLimit {
			return domain.ErrOperatorEmailChangeRateLimited
		}
		current, reReadErr := p.operators.FindOperator(txCtx, request.OperatorID)
		if reReadErr != nil {
			return reReadErr
		}
		if current.PasswordHash != provenHash {
			return domain.ErrOperatorPasswordMismatch
		}
		if _, findErr := p.operators.FindOperatorByEmail(txCtx, newEmail); findErr == nil {
			addressTaken = true
			return nil
		} else if !errors.Is(findErr, domain.ErrOperatorNotFound) {
			return fmt.Errorf("check operator email uniqueness: %w", findErr)
		}
		// Only now: a request that is going to be refused leaves the
		// operator's existing link usable.
		if revokeErr := p.tokens.RevokeEmailChanges(txCtx, request.OperatorID); revokeErr != nil {
			return revokeErr
		}
		created, createErr := p.tokens.CreateEmailChange(txCtx, domain.OperatorEmailChange{
			OperatorID: request.OperatorID,
			NewEmail:   newEmail,
			Token:      uuid.Must(uuid.NewV4()).String(),
			Expiry:     p.now().Add(p.expiry.EmailChange),
		})
		if createErr != nil {
			// A request that raced past the counted limit meets the
			// one-active-link index; the caller is told to wait either way.
			if errors.Is(createErr, domain.ErrOperatorEmailChangeActive) {
				return domain.ErrOperatorEmailChangeRateLimited
			}
			return createErr
		}
		change = created
		return nil
	}); err != nil {
		return err
	}

	if addressTaken {
		p.logger.Debug("email change skipped: address already in use",
			slog.Int64("operator_id", request.OperatorID),
			slog.String("new_email", maskedNewEmail),
		)
		return nil
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: request.OperatorID, Action: domain.OperatorAuditActionEmailChangeStarted,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &request.OperatorID, IPAddress: request.IPAddress,
		EmailChange: &domain.OperatorEmailChangeEvidence{MaskedNewEmail: maskedNewEmail},
	}, "email change initiation")

	p.changes.DispatchOperatorEmailChangeVerification(ctx, change)
	p.changes.DispatchOperatorEmailChangeRequested(ctx, operator, maskedNewEmail)
	return nil
}

// ConfirmOperatorEmailChange spends the link and writes the new address.
func (p *OperatorProvisioning) ConfirmOperatorEmailChange(ctx context.Context, token, ipAddress string) (domain.OperatorEmailChangeResult, error) {
	var result domain.OperatorEmailChangeResult
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		change, redeemErr := p.tokens.RedeemEmailChange(txCtx, token)
		if redeemErr != nil {
			return redeemErr
		}
		if _, findErr := p.operators.FindOperatorByEmail(txCtx, change.NewEmail); findErr == nil {
			return domain.ErrOperatorEmailInUse
		} else if !errors.Is(findErr, domain.ErrOperatorNotFound) {
			return fmt.Errorf("check operator email uniqueness: %w", findErr)
		}
		operator, operatorErr := p.operators.FindOperator(txCtx, change.OperatorID)
		if operatorErr != nil {
			if errors.Is(operatorErr, domain.ErrOperatorNotFound) {
				return domain.ErrOperatorEmailChangeNotFound
			}
			return operatorErr
		}
		if !operator.Active {
			return domain.ErrOperatorInactive
		}
		result = domain.OperatorEmailChangeResult{
			OperatorID: operator.ID, DisplayName: operator.DisplayName,
			OldEmail: operator.Email, NewEmail: change.NewEmail,
		}
		operator.Email = change.NewEmail
		if _, updateErr := p.operators.UpdateOperator(txCtx, operator); updateErr != nil {
			return updateErr
		}
		return nil
	}); err != nil {
		return domain.OperatorEmailChangeResult{}, err
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: result.OperatorID, Action: domain.OperatorAuditActionEmailChangeDone,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &result.OperatorID, IPAddress: ipAddress,
		EmailChange: &domain.OperatorEmailChangeEvidence{
			MaskedOldEmail: domain.MaskOperatorEmail(result.OldEmail),
			MaskedNewEmail: domain.MaskOperatorEmail(result.NewEmail),
		},
	}, "email change confirmation")

	// The address that was replaced learns the change went through, not just
	// that it was requested: for a compromised account that is the signal.
	p.changes.DispatchOperatorEmailChangeConfirmed(ctx, result.OldEmail, result.DisplayName)

	return result, nil
}

// CleanupOperatorEmailChanges spends the expired links so their operator can
// request a new one, then deletes the rows nobody counts any more.
func (p *OperatorProvisioning) CleanupOperatorEmailChanges(ctx context.Context) (int, error) {
	revoked, err := p.tokens.RevokeExpiredEmailChanges(ctx)
	if err != nil {
		return 0, fmt.Errorf("revoke expired email change links: %w", err)
	}
	if revoked > 0 {
		p.logger.Info("invalidated expired email change tokens", slog.Int("count", revoked))
	}
	return p.tokens.DeleteStaleEmailChanges(ctx)
}

// record appends the audit entry. A failed append is logged, never returned:
// the flow it describes has committed, and blocking it on the ledger would
// let an audit outage refuse legitimate operator work.
func (p *OperatorProvisioning) record(ctx context.Context, entry domain.OperatorAuditEntry, what string) {
	if err := p.audit.RecordOperatorAction(ctx, entry); err != nil {
		p.logger.Error("failed to create audit log for "+what,
			slog.Int64("operator_id", entry.OperatorID),
			slog.Any("error", err),
		)
	}
}
