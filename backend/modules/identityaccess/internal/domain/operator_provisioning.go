package domain

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

// The operator invitation and e-mail change flows (#3332). Both hand out a
// one-time link, so both are rate limited per actor and both re-check the
// state they decided on inside the transaction that spends the link.
const (
	// OperatorInvitationRateLimit caps the invitations one operator may
	// create within OperatorInvitationRateWindow. Without it an
	// authenticated operator could bulk-spam arbitrary addresses, which
	// amplifies into mass mail and Argon2id work on acceptance. A resend is
	// deliberately not counted: it reuses an existing row, is addressed to
	// the original invitee and is bounded by the pending invitations.
	OperatorInvitationRateLimit  = 5
	OperatorInvitationRateWindow = time.Hour

	// OperatorEmailChangeRateLimit caps the confirmation links one operator
	// may request within OperatorEmailChangeRateWindow.
	OperatorEmailChangeRateLimit  = 5
	OperatorEmailChangeRateWindow = time.Hour
)

var (
	// ErrOperatorEmailExists reports an address that already belongs to an
	// operator. The invitation reports it; the e-mail change deliberately
	// does not, so an operator cannot enumerate addresses.
	ErrOperatorEmailExists = errors.New("an operator with this email already exists")
	// ErrOperatorInvitationRateLimited reports an inviter that has created
	// too many invitations within the window.
	ErrOperatorInvitationRateLimited = errors.New("too many invitation attempts, please wait")
	// ErrOperatorEmailChangeRateLimited reports an operator that has
	// requested too many confirmation links within the window.
	ErrOperatorEmailChangeRateLimited = errors.New("too many email change attempts, please wait")
	// ErrOperatorEmailChangeSameEmail reports a requested address that is
	// the operator's current one.
	ErrOperatorEmailChangeSameEmail = errors.New("new email is the same as current email")
	// ErrOperatorEmailInUse reports a confirmation for an address another
	// operator took in the meantime.
	ErrOperatorEmailInUse = errors.New("email address is already in use")
)

// The rejection messages the operator surfaces show. They travel inside an
// InvalidInputError, whose text the retained envelope keeps, so these are
// part of the wire contract.
const (
	MessageOperatorEmailInvalid      = "invalid email format"
	MessageOperatorEmailTooLong      = "email address too long"
	MessageOperatorPasswordTooWeak   = "password doesn't meet complexity requirements"
	MessageOperatorDisplayNameNeeded = "display name is required"
)

// maxOperatorDisplayName is the column limit a display name must fit; the
// rejection names it so the caller can show the limit.
var messageOperatorDisplayNameTooLong = fmt.Sprintf("display name must not exceed %d characters", maxOperatorDisplayNameLength)

// NormalizeOperatorEmail lower-cases the address, canonicalizes it to the
// bare mailbox and applies the format and length rules. routable is the
// People Directory contact rule the composition binds: mail.ParseAddress
// alone accepts "t@t", which is not an address anyone can be reached at.
func NormalizeOperatorEmail(address string, routable func(string) bool) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(address))
	parsed, err := mail.ParseAddress(normalized)
	if err != nil {
		return "", InvalidInput(MessageOperatorEmailInvalid)
	}
	normalized = parsed.Address
	if routable != nil && !routable(normalized) {
		return "", InvalidInput(MessageOperatorEmailInvalid)
	}
	if len(normalized) > maxOperatorEmailLength {
		return "", InvalidInput(MessageOperatorEmailTooLong)
	}
	return normalized, nil
}

// NormalizeOperatorDisplayName trims the name and applies the length rule.
func NormalizeOperatorDisplayName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return "", InvalidInput(MessageOperatorDisplayNameNeeded)
	case len(trimmed) > maxOperatorDisplayNameLength:
		return "", InvalidInput(messageOperatorDisplayNameTooLong)
	}
	return trimmed, nil
}

// OperatorInvitationRequest is one invitation to create.
type OperatorInvitationRequest struct {
	Email       string
	DisplayName *string
	CreatedBy   int64
	IPAddress   string
}

// OperatorInvitationPreview is what the public validation page may show.
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

// OperatorEmailChangeResult is what a confirmation produced: the address
// that is now the operator's, and the one it replaced.
type OperatorEmailChangeResult struct {
	OperatorID  int64
	DisplayName string
	OldEmail    string
	NewEmail    string
}
