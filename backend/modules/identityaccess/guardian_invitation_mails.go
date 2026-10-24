package identityaccess

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// The guardian invitation mails (#2722, #3320, #3534): what the flows queue
// and what the queued mail says when Delivery renders it. The root stores a
// GuardianMail in the outbox payload and reads it back.

// GuardianInvitationExpiryFallback is retained for legacy mail payloads without
// a recorded lifetime and for read-only fixture compositions. New links use
// the resolved tenant setting exclusively.
const GuardianInvitationExpiryFallback = 48 * time.Hour

// GuardianInvitationExpiry converts the resolved school setting to a lifetime.
// Invalid configuration must not silently create a link with another lifetime.
func GuardianInvitationExpiry(settingHours int) (time.Duration, error) {
	if settingHours <= 0 {
		return 0, errors.New("guardian invitation lifetime must be positive")
	}
	return time.Duration(settingHours) * time.Hour, nil
}

// GuardianMailRecipient is the guardian a guardian mail addresses.
type GuardianMailRecipient struct {
	FirstName string
	LastName  string
	Email     string
}

// GuardianMail is what a queued guardian mail names: the invitation with its
// accept link, the variant for an address that already owns an account, or
// the welcome that follows either.
type GuardianMail struct {
	Recipient     string
	FirstName     string
	LastName      string
	InvitationURL string
	LogoURL       string
	SchoolName    string
	// ExpiryHours is the "valid for N hours" of the invitation.
	ExpiryHours int
	// ExistingAccount marks the variant whose link is the parents portal
	// login, asking for the existing credentials (#3320).
	ExistingAccount bool
	// HelpURL, ParentInfoURL and PrecedingOutboxID belong to the welcome
	// (#3534): the help article, the Elterninfo and the queued mail it
	// waits for.
	HelpURL           string
	ParentInfoURL     string
	PrecedingOutboxID int64
}

// guardianMail fills what every guardian mail shares. linkPath is appended
// to the parents portal origin.
func guardianMail(recipient GuardianMailRecipient, origin, linkPath, schoolName string) GuardianMail {
	origin = MailOrigin(origin)
	return GuardianMail{
		Recipient:     strings.TrimSpace(recipient.Email),
		FirstName:     strings.TrimSpace(recipient.FirstName),
		LastName:      strings.TrimSpace(recipient.LastName),
		InvitationURL: origin + linkPath,
		LogoURL:       origin + motoLogoPath,
		SchoolName:    schoolName,
	}
}

// GuardianInvitationMailOf is the mail carrying the accept link. expiresAt
// becomes the "valid for N hours" the mail states, floored at one hour so a
// link that is about to expire never advertises zero.
func GuardianInvitationMailOf(recipient GuardianMailRecipient, origin, token string, expiresAt time.Time, schoolName string) GuardianMail {
	mail := guardianMail(recipient, origin, "/accept-guardian-invite/"+token, schoolName)
	mail.ExpiryHours = max(int(time.Until(expiresAt)/time.Hour), 1)
	return mail
}

// GuardianExistingAccountMailOf is the variant for an address that already
// owns an account (#3320): no registration and no token, just the parents
// portal login and a hint to use the existing credentials. A blank address
// gets no mail.
func GuardianExistingAccountMailOf(recipient GuardianMailRecipient, origin, schoolName string) (GuardianMail, bool) {
	if strings.TrimSpace(recipient.Email) == "" {
		return GuardianMail{}, false
	}
	mail := guardianMail(recipient, origin, "/login", schoolName)
	mail.ExistingAccount = true
	return mail, true
}

// errGuardianWelcomePredecessorRequired refuses a welcome that names no
// preceding mail: it could overtake the mail that granted the access.
var errGuardianWelcomePredecessorRequired = errors.New("guardian welcome email: preceding outbox ID is required")

// GuardianWelcomeMailOf is the welcome that follows a new access (#3534):
// the help article for parents and the Elterninfo, carrying no accept link,
// released once the mail in outbox row precedingOutboxID was accepted. A
// blank address gets no mail.
func GuardianWelcomeMailOf(recipient GuardianMailRecipient, origin, schoolName, helpURL, parentInfoURL string, precedingOutboxID int64) (GuardianMail, bool, error) {
	if precedingOutboxID <= 0 {
		return GuardianMail{}, false, errGuardianWelcomePredecessorRequired
	}
	if strings.TrimSpace(recipient.Email) == "" {
		return GuardianMail{}, false, nil
	}
	mail := guardianMail(recipient, origin, "/login", schoolName)
	mail.InvitationURL = ""
	mail.HelpURL, mail.ParentInfoURL, mail.PrecedingOutboxID = helpURL, parentInfoURL, precedingOutboxID
	return mail, true, nil
}

// GuardianWelcomeIdempotencyKey names the guardian contact, which belongs to
// one school, so a guardian is welcomed once per school however often they
// are invited or linked to a child.
func GuardianWelcomeIdempotencyKey(guardianProfileID int64) string {
	return fmt.Sprintf("guardian_welcome:%d", guardianProfileID)
}

// GuardianInvitationMail renders a queued invitation or existing-account
// mail.
func GuardianInvitationMail(mail GuardianMail) (Mail, error) {
	if mail.Recipient == "" {
		return Mail{}, errors.New("guardian invitation payload missing recipient_email")
	}
	if mail.InvitationURL == "" {
		return Mail{}, errors.New("guardian invitation payload missing invitation_url")
	}
	expiryHours := mail.ExpiryHours
	if expiryHours <= 0 {
		expiryHours = int(GuardianInvitationExpiryFallback / time.Hour)
	}
	subject := "Einladung zum Eltern-Portal"
	if mail.ExistingAccount {
		subject = "Ihr Zugang zum Eltern-Portal"
	}
	return Mail{
		Subject:  schoolSubject(subject, mail.SchoolName),
		Template: "guardian-invitation.html",
		Content: map[string]any{
			"InvitationURL":   mail.InvitationURL,
			"FirstName":       mail.FirstName,
			"LastName":        mail.LastName,
			"ExpiryHours":     expiryHours,
			"LogoURL":         mail.LogoURL,
			"SchoolName":      mail.SchoolName,
			"ExistingAccount": mail.ExistingAccount,
		},
	}, nil
}

// GuardianWelcomeMail renders a queued welcome.
func GuardianWelcomeMail(mail GuardianMail) (Mail, error) {
	if mail.Recipient == "" {
		return Mail{}, errors.New("guardian welcome payload missing recipient_email")
	}
	if mail.HelpURL == "" {
		return Mail{}, errors.New("guardian welcome payload missing help_url")
	}
	return Mail{
		Subject:  schoolSubject("Willkommen bei moto", mail.SchoolName),
		Template: "guardian-welcome.html",
		Content: map[string]any{
			"HelpURL":       mail.HelpURL,
			"ParentInfoURL": mail.ParentInfoURL,
			"FirstName":     mail.FirstName,
			"LastName":      mail.LastName,
			"LogoURL":       mail.LogoURL,
			"SchoolName":    mail.SchoolName,
		},
	}, nil
}

// MailRelease is whether a queued follow-up mail may be rendered now.
type MailRelease int

const (
	// MailReleased renders the mail.
	MailReleased MailRelease = iota
	// MailDeferred keeps the mail queued until its predecessor settles.
	MailDeferred
	// MailCancelled drops the mail: its predecessor can no longer arrive.
	MailCancelled
)

// GuardianWelcomeRelease renders the welcome only after the mail that
// granted the access was accepted, and never once that mail can no longer
// arrive.
func GuardianWelcomeRelease(precedingSent, precedingTerminal bool) MailRelease {
	switch {
	case precedingTerminal:
		return MailCancelled
	case !precedingSent:
		return MailDeferred
	default:
		return MailReleased
	}
}
