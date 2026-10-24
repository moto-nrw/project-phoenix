package domain

import (
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

// NormalizeOperatorEmail lower-cases the address, canonicalizes it to the
// bare mailbox and applies the format and length rules. routable is the
// People Directory contact rule the composition binds: mail.ParseAddress
// alone accepts "t@t", which is not an address anyone can be reached at.
func NormalizeOperatorEmail(address string, routable func(string) bool) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(address))
	parsed, err := mail.ParseAddress(normalized)
	if err != nil {
		return "", &InvalidInputError{Err: ErrOperatorEmailInvalid}
	}
	normalized = parsed.Address
	if routable != nil && !routable(normalized) {
		return "", &InvalidInputError{Err: ErrOperatorEmailInvalid}
	}
	if len(normalized) > maxOperatorEmailLength {
		return "", &InvalidInputError{Err: ErrOperatorEmailTooLong}
	}
	return normalized, nil
}

// NormalizeOperatorDisplayName trims the name and applies the length rule.
func NormalizeOperatorDisplayName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return "", &InvalidInputError{Err: ErrOperatorDisplayNameRequired}
	case len(trimmed) > maxOperatorDisplayNameLength:
		return "", &InvalidInputError{Err: ErrOperatorDisplayNameTooLong}
	}
	return trimmed, nil
}
