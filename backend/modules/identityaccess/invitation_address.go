package identityaccess

import (
	"errors"
	"net/mail"
	"strings"
)

var (
	// ErrInvitationEmailRequired: the invitation names no e-mail address.
	ErrInvitationEmailRequired = errors.New("email is required")
	// ErrInvitationEmailInvalid: the address is not a plain address of at
	// most maxInvitationEmailLength characters.
	ErrInvitationEmailInvalid = errors.New("invalid email address")
)

// maxInvitationEmailLength is the longest address an invitation takes
// (RFC 5321).
const maxInvitationEmailLength = 254

// NormalizeInvitationEmail is the address an invitation stores: trimmed, in
// lower case and a plain address. A display name ("Lea <lea@example.de>")
// is refused rather than stripped, so the stored address is exactly what
// the caller typed.
func NormalizeInvitationEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", ErrInvitationEmailRequired
	}
	if len(email) > maxInvitationEmailLength {
		return "", ErrInvitationEmailInvalid
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", ErrInvitationEmailInvalid
	}
	return email, nil
}

// PendingInvitationsByPerson indexes the school's open invitations by the
// person they name; an invitation naming no person is left out.
func PendingInvitationsByPerson(pending []SchoolInvitation) map[int64][]SchoolInvitation {
	byPerson := make(map[int64][]SchoolInvitation, len(pending))
	for _, invitation := range pending {
		if invitation.PersonID != nil {
			byPerson[*invitation.PersonID] = append(byPerson[*invitation.PersonID], invitation)
		}
	}
	return byPerson
}
