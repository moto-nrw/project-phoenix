package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
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
	attach    func(context.Context) context.Context
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
	// Attach binds the composed unit of work ahead of every public flow.
	Attach func(context.Context) context.Context
	Expiry OperatorProvisioningExpiry
	Logger *slog.Logger
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
		runtime: deps.Runtime, attach: deps.Attach, expiry: deps.Expiry, now: time.Now,
		logger: logger.With("component", "operator-provisioning"),
	}, nil
}

// --- invitation ------------------------------------------------------------

// InviteOperator creates the link that lets the address become an operator
// and mails it.
func (p *OperatorProvisioning) InviteOperator(ctx context.Context, request identityaccess.OperatorInvitationRequest) (err error) {
	if p == nil {
		return errOperatorProvisioningUnavailable
	}
	defer func() { err = operatorProvisioningError(err) }()
	ctx = bindRuntime(p.attach, ctx)
	email, err := domain.NormalizeOperatorEmail(request.Email, p.format.IsRoutable)
	if err != nil {
		return err
	}

	// An address that already is an operator is reported: an invitation is
	// an administrative act, not something an unauthenticated caller does,
	// so there is nothing to enumerate here.
	if _, err := p.operators.FindOperatorByEmail(ctx, email); err == nil {
		return identityaccess.ErrOperatorEmailExists
	} else if !errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return fmt.Errorf("check operator email uniqueness: %w", err)
	}

	var invitation domain.OperatorInvitation
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		var err error
		invitation, err = p.createInvitationInAdmin(txCtx, request, email)
		return err
	}); err != nil {
		return err
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: request.CreatedBy, Action: domain.OperatorAuditActionInvitationCreated,
		ResourceType: domain.OperatorAuditResourceInvitation, ResourceID: &invitation.ID, IPAddress: request.IPAddress,
	}, "operator invitation")

	return nil
}

// createInvitationInAdmin applies the rate limit, spends the address's
// earlier links and creates and dispatches the new one inside the
// administrative transaction.
func (p *OperatorProvisioning) createInvitationInAdmin(txCtx context.Context, request identityaccess.OperatorInvitationRequest, email string) (domain.OperatorInvitation, error) {
	// Lock the inviter's row so two parallel invitations from the same
	// operator serialize: under READ COMMITTED both would otherwise read
	// the same count and both pass the limit.
	if _, lockErr := p.operators.findOperatorForUpdate(txCtx, request.CreatedBy); lockErr != nil {
		return domain.OperatorInvitation{}, fmt.Errorf("lock inviter row: %w", lockErr)
	}
	recent, countErr := p.tokens.CountInvitationsCreatedAfter(txCtx, request.CreatedBy, p.now().Add(-domain.OperatorInvitationRateWindow))
	if countErr != nil {
		return domain.OperatorInvitation{}, fmt.Errorf("check invitation rate limit: %w", countErr)
	}
	if recent >= domain.OperatorInvitationRateLimit {
		return domain.OperatorInvitation{}, identityaccess.ErrOperatorInvitationRateLimited
	}
	if _, revokeErr := p.tokens.RevokeInvitationsForEmail(txCtx, email); revokeErr != nil {
		return domain.OperatorInvitation{}, fmt.Errorf("revoke previous invitations: %w", revokeErr)
	}
	created, createErr := p.tokens.CreateInvitation(txCtx, domain.OperatorInvitation{
		Email:       email,
		Token:       uuid.Must(uuid.NewV4()).String(),
		ExpiresAt:   p.now().Add(p.expiry.Invitation),
		CreatedBy:   request.CreatedBy,
		DisplayName: request.DisplayName,
	})
	if createErr != nil {
		return domain.OperatorInvitation{}, createErr
	}
	return created, p.invites.DispatchOperatorInvitation(txCtx, created, p.inviterName(txCtx, request.CreatedBy))
}

// ValidateOperatorInvitation returns what the public accept page may show
// about a still redeemable link.
func (p *OperatorProvisioning) ValidateOperatorInvitation(ctx context.Context, token string) (identityaccess.OperatorInvitationPreview, error) {
	if p == nil {
		return identityaccess.OperatorInvitationPreview{}, errOperatorProvisioningUnavailable
	}
	invitation, err := p.tokens.FindRedeemableInvitation(bindRuntime(p.attach, ctx), token)
	if err != nil {
		return identityaccess.OperatorInvitationPreview{}, operatorProvisioningError(err)
	}
	return identityaccess.OperatorInvitationPreview{
		Email: invitation.Email, DisplayName: invitation.DisplayName, ExpiresAt: invitation.ExpiresAt,
	}, nil
}

// AcceptOperatorInvitation creates the operator the link promises.
func (p *OperatorProvisioning) AcceptOperatorInvitation(ctx context.Context, acceptance identityaccess.OperatorInvitationAcceptance) (_ identityaccess.Operator, err error) {
	if p == nil {
		return identityaccess.Operator{}, errOperatorProvisioningUnavailable
	}
	defer func() { err = operatorProvisioningError(err) }()
	ctx = bindRuntime(p.attach, ctx)
	// Refuse the cheap mistakes before any database work.
	if err := p.hasher.ValidatePasswordStrength(acceptance.Password); err != nil {
		return identityaccess.Operator{}, &domain.InvalidInputError{Err: domain.ErrOperatorPasswordTooWeak}
	}
	displayName, err := domain.NormalizeOperatorDisplayName(acceptance.DisplayName)
	if err != nil {
		return identityaccess.Operator{}, err
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
			return identityaccess.ErrOperatorEmailExists
		} else if !errors.Is(findErr, identityaccess.ErrOperatorNotFound) {
			return fmt.Errorf("check operator email uniqueness: %w", findErr)
		}
		hash, hashErr := p.hasher.HashPassword(acceptance.Password)
		if hashErr != nil {
			return fmt.Errorf("hash operator password: %w", hashErr)
		}
		created, createErr := p.operators.CreateOperator(txCtx, identityaccess.Operator{
			Email: invitation.Email, DisplayName: displayName, PasswordHash: hash, Active: true,
		})
		if createErr != nil {
			return createErr
		}
		operator = domain.Operator(created)
		return nil
	}); err != nil {
		return identityaccess.Operator{}, err
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: operator.ID, Action: domain.OperatorAuditActionInvitationAccepted,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &operator.ID, IPAddress: acceptance.IPAddress,
	}, "operator invitation acceptance")

	return identityaccess.Operator(operator), nil
}

// RevokeOperatorInvitationByActor spends a redeemable link without
// redeeming it and records the actor in the operator ledger.
func (p *OperatorProvisioning) RevokeOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if p == nil {
		return errOperatorProvisioningUnavailable
	}
	ctx = bindRuntime(p.attach, ctx)
	if err := p.tokens.RevokeInvitation(ctx, invitationID); err != nil {
		return operatorProvisioningError(err)
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
	return p.runtime.WithAdminTx(ctx, func(ctx context.Context) error {
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

		if err := p.invites.DispatchOperatorInvitation(ctx, invitation, p.inviterName(ctx, actorID)); err != nil {
			return err
		}

		p.record(ctx, domain.OperatorAuditEntry{
			OperatorID: actorID, Action: domain.OperatorAuditActionInvitationResent,
			ResourceType: domain.OperatorAuditResourceInvitation, ResourceID: &invitationID, IPAddress: ipAddress,
		}, "invitation resend")
		return nil
	})
}

// inviterName reads the display name the invitation mail shows. A failure is
// not worth refusing the invitation over: the mail falls back to the generic
// wording the template carries.
func (p *OperatorProvisioning) inviterName(ctx context.Context, inviterID int64) string {
	inviter, err := p.operators.findOperatorByID(ctx, inviterID)
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
func (p *OperatorProvisioning) InitiateOperatorEmailChange(ctx context.Context, request identityaccess.OperatorEmailChangeRequest) (err error) {
	if p == nil {
		return errOperatorProvisioningUnavailable
	}
	defer func() { err = operatorProvisioningError(err) }()
	ctx = bindRuntime(p.attach, ctx)
	operator, err := p.operators.findOperatorByID(ctx, request.OperatorID)
	if err != nil {
		return err
	}
	if !operator.Active {
		return identityaccess.ErrOperatorInactive
	}
	// Argon2id is expensive, so it runs before the transaction is opened.
	match, verifyErr := p.passwords.VerifyPassword(request.CurrentPassword, operator.PasswordHash)
	if verifyErr != nil || !match {
		return identityaccess.ErrOperatorPasswordMismatch
	}
	// The hash the check accepted, so a password rotation that commits in
	// the meantime is detected inside the transaction without a second hash.
	provenHash := operator.PasswordHash

	newEmail, err := domain.NormalizeOperatorEmail(request.NewEmail, p.format.IsRoutable)
	if err != nil {
		return err
	}
	if newEmail == operator.Email {
		return identityaccess.ErrOperatorEmailChangeSameEmail
	}
	maskedNewEmail := domain.MaskEmail(newEmail)

	var change domain.OperatorEmailChange
	var addressTaken bool
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		var err error
		addressTaken, err = p.prepareOperatorEmailChange(txCtx, request, operator, provenHash, newEmail, maskedNewEmail, &change)
		return err
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

	return nil
}

// ConfirmOperatorEmailChange spends the link and writes the new address.
func (p *OperatorProvisioning) ConfirmOperatorEmailChange(ctx context.Context, token, ipAddress string) (_ identityaccess.OperatorEmailChangeResult, err error) {
	if p == nil {
		return identityaccess.OperatorEmailChangeResult{}, errOperatorProvisioningUnavailable
	}
	defer func() { err = operatorProvisioningError(err) }()
	ctx = bindRuntime(p.attach, ctx)
	var result identityaccess.OperatorEmailChangeResult
	if err := p.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = p.confirmEmailChangeInAdmin(txCtx, token)
		return err
	}); err != nil {
		return identityaccess.OperatorEmailChangeResult{}, err
	}

	p.record(ctx, domain.OperatorAuditEntry{
		OperatorID: result.OperatorID, Action: domain.OperatorAuditActionEmailChangeDone,
		ResourceType: domain.OperatorAuditResourceOperator, ResourceID: &result.OperatorID, IPAddress: ipAddress,
		EmailChange: &domain.OperatorEmailChangeEvidence{
			MaskedOldEmail: domain.MaskEmail(result.OldEmail),
			MaskedNewEmail: domain.MaskEmail(result.NewEmail),
		},
	}, "email change confirmation")

	// The address that was replaced learns the change went through, not just
	// that it was requested: for a compromised account that is the signal.

	return result, nil
}

// confirmEmailChangeInAdmin spends the link and writes the new address inside
// the administrative transaction.
func (p *OperatorProvisioning) confirmEmailChangeInAdmin(txCtx context.Context, token string) (identityaccess.OperatorEmailChangeResult, error) {
	change, redeemErr := p.tokens.RedeemEmailChange(txCtx, token)
	if redeemErr != nil {
		return identityaccess.OperatorEmailChangeResult{}, redeemErr
	}
	if _, findErr := p.operators.FindOperatorByEmail(txCtx, change.NewEmail); findErr == nil {
		return identityaccess.OperatorEmailChangeResult{}, identityaccess.ErrOperatorEmailInUse
	} else if !errors.Is(findErr, identityaccess.ErrOperatorNotFound) {
		return identityaccess.OperatorEmailChangeResult{}, fmt.Errorf("check operator email uniqueness: %w", findErr)
	}
	operator, operatorErr := p.operators.findOperatorByID(txCtx, change.OperatorID)
	if operatorErr != nil {
		if errors.Is(operatorErr, identityaccess.ErrOperatorNotFound) {
			return identityaccess.OperatorEmailChangeResult{}, identityaccess.ErrOperatorEmailChangeNotFound
		}
		return identityaccess.OperatorEmailChangeResult{}, operatorErr
	}
	if !operator.Active {
		return identityaccess.OperatorEmailChangeResult{}, identityaccess.ErrOperatorInactive
	}
	result := identityaccess.OperatorEmailChangeResult{
		OperatorID: operator.ID, DisplayName: operator.DisplayName,
		OldEmail: operator.Email, NewEmail: change.NewEmail,
	}
	operator.Email = change.NewEmail
	if _, updateErr := p.operators.UpdateOperator(txCtx, identityaccess.Operator(operator)); updateErr != nil {
		return identityaccess.OperatorEmailChangeResult{}, updateErr
	}
	return result, p.changes.DispatchOperatorEmailChangeConfirmed(txCtx, result.OldEmail, result.DisplayName)
}

// CleanupOperatorEmailChanges spends the expired links so their operator can
// request a new one, then deletes the rows nobody counts any more.
func (p *OperatorProvisioning) CleanupOperatorEmailChanges(ctx context.Context) (_ int, err error) {
	if p == nil {
		return 0, errOperatorProvisioningUnavailable
	}
	defer func() { err = operatorProvisioningError(err) }()
	ctx = bindRuntime(p.attach, ctx)
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

type OperatorInvitationDelivery struct {
	Source identityPorts.OperatorInvitationDelivery
	Expiry time.Duration
}

func (d OperatorInvitationDelivery) DispatchOperatorInvitation(ctx context.Context, invitation domain.OperatorInvitation, inviterName string) error {
	return d.Source.DispatchOperatorInvitation(ctx, PublicOperatorInvitation(invitation), inviterName, d.Expiry)
}

type OperatorEmailChangeDelivery struct {
	identityPorts.OperatorEmailChangeDelivery
	Expiry time.Duration
}

func (d OperatorEmailChangeDelivery) DispatchOperatorEmailChangeVerification(ctx context.Context, change domain.OperatorEmailChange) error {
	return d.OperatorEmailChangeDelivery.DispatchOperatorEmailChangeVerification(ctx, PublicOperatorEmailChange(change), d.Expiry)
}

func (d OperatorEmailChangeDelivery) DispatchOperatorEmailChangeRequested(ctx context.Context, operator domain.Operator, maskedNewEmail string) error {
	return d.OperatorEmailChangeDelivery.DispatchOperatorEmailChangeRequested(ctx, identityaccess.Operator(operator), maskedNewEmail)
}

func (p *OperatorProvisioning) enqueueEmailChangeNotices(ctx context.Context, change domain.OperatorEmailChange, operator domain.Operator, maskedNewEmail string) error {
	if err := p.changes.DispatchOperatorEmailChangeVerification(ctx, change); err != nil {
		return err
	}
	return p.changes.DispatchOperatorEmailChangeRequested(ctx, operator, maskedNewEmail)
}

func (p *OperatorProvisioning) prepareOperatorEmailChange(txCtx context.Context, request identityaccess.OperatorEmailChangeRequest, operator domain.Operator, provenHash, newEmail, maskedNewEmail string, change *domain.OperatorEmailChange) (bool, error) {
	// Lock the operator's row so two parallel requests serialize instead
	// of both relying on the partial unique index to catch the conflict.
	if _, lockErr := p.operators.findOperatorForUpdate(txCtx, request.OperatorID); lockErr != nil {
		return false, fmt.Errorf("lock operator row: %w", lockErr)
	}
	recent, countErr := p.tokens.CountEmailChangesCreatedAfter(txCtx, request.OperatorID, p.now().Add(-domain.OperatorEmailChangeRateWindow))
	if countErr != nil {
		return false, fmt.Errorf("check email change rate limit: %w", countErr)
	}
	if recent >= domain.OperatorEmailChangeRateLimit {
		return false, identityaccess.ErrOperatorEmailChangeRateLimited
	}
	current, reReadErr := p.operators.findOperatorByID(txCtx, request.OperatorID)
	if reReadErr != nil {
		return false, reReadErr
	}
	if current.PasswordHash != provenHash {
		return false, identityaccess.ErrOperatorPasswordMismatch
	}
	if _, findErr := p.operators.FindOperatorByEmail(txCtx, newEmail); findErr == nil {
		return true, nil
	} else if !errors.Is(findErr, identityaccess.ErrOperatorNotFound) {
		return false, fmt.Errorf("check operator email uniqueness: %w", findErr)
	}
	// Only now: a request that is going to be refused leaves the
	// operator's existing link usable.
	if revokeErr := p.tokens.RevokeEmailChanges(txCtx, request.OperatorID); revokeErr != nil {
		return false, revokeErr
	}
	created, createErr := p.tokens.CreateEmailChange(txCtx, domain.OperatorEmailChange{
		OperatorID: request.OperatorID,
		NewEmail:   newEmail,
		Token:      uuid.Must(uuid.NewV4()).String(),
		Expiry:     p.now().Add(p.expiry.EmailChange),
	})
	if createErr != nil {
		return false, createErr
	}
	*change = created
	return false, p.enqueueEmailChangeNotices(txCtx, *change, operator, maskedNewEmail)
}

func (p *OperatorProvisioning) ResendOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	if p == nil {
		return errOperatorProvisioningUnavailable
	}
	return operatorProvisioningError(p.ResendOperatorInvitation(bindRuntime(p.attach, ctx), invitationID, actorID, ipAddress))
}

func (p *OperatorProvisioning) ListPendingOperatorInvitations(ctx context.Context) ([]identityaccess.OperatorInvitation, error) {
	if p == nil {
		return nil, errOperatorProvisioningUnavailable
	}
	invitations, err := p.tokens.ListRedeemableInvitations(bindRuntime(p.attach, ctx))
	if err != nil {
		return nil, operatorProvisioningError(err)
	}
	result := make([]identityaccess.OperatorInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, PublicOperatorInvitation(invitation))
	}
	return result, nil
}

var errOperatorProvisioningUnavailable = identityaccess.ErrOperatorProvisioningUnavailable

// operatorProvisioningSentinels are the invitation, e-mail change and status
// outcomes of these flows, reported as public sentinels already. A match
// keeps the error as it is; everything else falls through to the operator
// mapping, which carries the operator and credential sentinels these flows
// share. The link sentinels belong here because the operator mapping does
// not carry them; the storage capability reports them through MapError.
var operatorProvisioningSentinels = []error{
	identityaccess.ErrOperatorEmailExists,
	identityaccess.ErrOperatorInvitationRateLimited,
	identityaccess.ErrOperatorEmailChangeRateLimited,
	identityaccess.ErrOperatorEmailChangeSameEmail,
	identityaccess.ErrOperatorEmailInUse,
	identityaccess.ErrOperatorSelfDeactivation,
	identityaccess.ErrOperatorInvitationNotFound,
	identityaccess.ErrOperatorEmailChangeNotFound,
}

// operatorProvisioningError translates a flow error to the public contract.
func operatorProvisioningError(err error) error {
	if err == nil {
		return nil
	}
	for _, sentinel := range operatorProvisioningSentinels {
		if errors.Is(err, sentinel) {
			return err
		}
	}
	return operatorError(err)
}
