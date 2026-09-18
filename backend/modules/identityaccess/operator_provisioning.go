package identityaccess

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	// ErrOperatorEmailExists reports an address that already belongs to an
	// operator. The invitation reports it; the e-mail change deliberately
	// answers success instead, so an authenticated operator cannot use that
	// endpoint as an operator directory.
	ErrOperatorEmailExists = errors.New("an operator with this email already exists")
	// ErrOperatorInvitationRateLimited reports an inviter past the limit of
	// five invitations per hour.
	ErrOperatorInvitationRateLimited = errors.New("too many invitation attempts, please wait")
	// ErrOperatorEmailChangeRateLimited reports an operator past the limit
	// of five confirmation links per hour.
	ErrOperatorEmailChangeRateLimited = errors.New("too many email change attempts, please wait")
	// ErrOperatorEmailChangeSameEmail reports a requested address that is
	// already the operator's.
	ErrOperatorEmailChangeSameEmail = errors.New("new email is the same as current email")
	// ErrOperatorEmailInUse reports a confirmation for an address another
	// operator took in the meantime.
	ErrOperatorEmailInUse = errors.New("email address is already in use")
	// ErrOperatorProvisioningUnavailable reports a composition without the
	// operator dependencies the provisioning flows need.
	ErrOperatorProvisioningUnavailable = errors.New("operator provisioning is not composed")
)

// OperatorInvitationRequest is one invitation to create. IPAddress is
// recorded in the operator ledger.
type OperatorInvitationRequest struct {
	Email       string
	DisplayName *string
	CreatedBy   int64
	IPAddress   string
}

// OperatorInvitationPreview is what the public accept page may show about a
// still redeemable link.
type OperatorInvitationPreview struct {
	Email       string
	DisplayName *string
	ExpiresAt   time.Time
}

// OperatorInvitationAcceptance is what an invitee supplies.
type OperatorInvitationAcceptance struct {
	Token       string
	DisplayName string
	Password    string
	IPAddress   string
}

// OperatorEmailChangeRequest is one confirmation link to create.
type OperatorEmailChangeRequest struct {
	OperatorID      int64
	NewEmail        string
	CurrentPassword string
	IPAddress       string
}

// OperatorEmailChangeResult is what a confirmation produced.
type OperatorEmailChangeResult struct {
	OperatorID  int64
	DisplayName string
	OldEmail    string
	NewEmail    string
}

// OperatorProvisioning is the operator invitation and e-mail change
// capability the operator management screens and the scheduler consume
// (#3332).
//
// Both flows hand out a one-time link and both are rate limited per actor:
// five per hour. A refusal never spends the actor's existing link, an
// acceptance spends the token before it hashes a password, and every flow
// records its own entry in the operator ledger. Refusals carry
// ErrOperatorInvitationNotFound, ErrOperatorEmailChangeNotFound,
// ErrOperatorEmailExists, ErrOperatorEmailInUse, ErrOperatorNotFound,
// ErrOperatorInactive, ErrOperatorPasswordMismatch, one of the two rate
// limits, ErrOperatorEmailChangeSameEmail, or an InvalidInputError whose
// message the caller may show.
type OperatorProvisioning interface {
	InviteOperator(ctx context.Context, request OperatorInvitationRequest) error
	ValidateOperatorInvitation(ctx context.Context, token string) (OperatorInvitationPreview, error)
	AcceptOperatorInvitation(ctx context.Context, acceptance OperatorInvitationAcceptance) (Operator, error)
	// ListPendingOperatorInvitations returns the links that can still be
	// spent, newest first.
	ListPendingOperatorInvitations(ctx context.Context) ([]OperatorInvitation, error)
	RevokeOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error
	ResendOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error

	InitiateOperatorEmailChange(ctx context.Context, request OperatorEmailChangeRequest) error
	ConfirmOperatorEmailChange(ctx context.Context, token, ipAddress string) (OperatorEmailChangeResult, error)
	// CleanupOperatorEmailChanges spends the expired links so their operator
	// can request a new one, then deletes the rows nobody counts any more.
	CleanupOperatorEmailChanges(ctx context.Context) (int, error)
}

func (m *Module) InviteOperator(ctx context.Context, request OperatorInvitationRequest) error {
	return m.engine.InviteOperator(ctx, request)
}

func (m *Module) ValidateOperatorInvitation(ctx context.Context, token string) (OperatorInvitationPreview, error) {
	return m.engine.ValidateOperatorInvitation(ctx, token)
}

func (m *Module) AcceptOperatorInvitation(ctx context.Context, acceptance OperatorInvitationAcceptance) (Operator, error) {
	return m.engine.AcceptOperatorInvitation(ctx, acceptance)
}

func (m *Module) ListPendingOperatorInvitations(ctx context.Context) ([]OperatorInvitation, error) {
	return m.engine.ListPendingOperatorInvitations(ctx)
}

func (m *Module) RevokeOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	return m.engine.RevokeOperatorInvitationByActor(ctx, invitationID, actorID, ipAddress)
}

func (m *Module) ResendOperatorInvitationByActor(ctx context.Context, invitationID, actorID int64, ipAddress string) error {
	return m.engine.ResendOperatorInvitationByActor(ctx, invitationID, actorID, ipAddress)
}

func (m *Module) InitiateOperatorEmailChange(ctx context.Context, request OperatorEmailChangeRequest) error {
	return m.engine.InitiateOperatorEmailChange(ctx, request)
}

func (m *Module) ConfirmOperatorEmailChange(ctx context.Context, token, ipAddress string) (OperatorEmailChangeResult, error) {
	return m.engine.ConfirmOperatorEmailChange(ctx, token, ipAddress)
}

func (m *Module) CleanupOperatorEmailChanges(ctx context.Context) (int, error) {
	return m.engine.CleanupOperatorEmailChanges(ctx)
}

// MaskOperatorEmail masks an address for a log line: the first character
// plus *** before the @. A local part of one or two characters is hidden
// entirely, because one character of two would reveal half the name.
func MaskOperatorEmail(address string) string {
	parts := strings.SplitN(address, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	if len(parts[0]) <= 2 {
		return "***@" + parts[1]
	}
	return string(parts[0][0]) + "***@" + parts[1]
}
