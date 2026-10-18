package identityaccess

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// The guardian invitation mails (#2722, #3320, #3534): the queued payload the
// flows write and what the queued mail says when Delivery renders it.

// The outbox payload keys of the guardian invitation and welcome mails.
const (
	GuardianPayloadRecipientEmail = "recipient_email"
	GuardianPayloadFirstName      = "first_name"
	GuardianPayloadLastName       = "last_name"
	GuardianPayloadInvitationURL  = "invitation_url"
	GuardianPayloadLogoURL        = "logo_url"
	GuardianPayloadSchoolName     = "school_name"
	GuardianPayloadExpiryHours    = "expiry_hours"
	// GuardianPayloadExistingAccount marks the variant for an address that
	// already owns an account: invitation_url is then the parents portal
	// login, and the mail asks for the existing credentials (#3320).
	GuardianPayloadExistingAccount = "existing_account"
	// The welcome mail (#3534) carries the help article and the Elterninfo
	// instead of an accept link.
	GuardianPayloadHelpURL           = "help_url"
	GuardianPayloadParentInfoURL     = "parent_info_url"
	GuardianPayloadPrecedingOutboxID = "preceding_outbox_id"
)

// GuardianInvitationExpiryFallback applies when neither the school setting
// nor the deployment name a guardian token lifetime. 48 hours matches the
// staff invitation default and the registry default.
const GuardianInvitationExpiryFallback = 48 * time.Hour

// GuardianInvitationExpiry is the lifetime of a guardian link: the school's
// setting in hours, then the lifetime the deployment configured, then the
// fallback.
func GuardianInvitationExpiry(settingHours int, deployment time.Duration) time.Duration {
	if settingHours > 0 {
		return time.Duration(settingHours) * time.Hour
	}
	if deployment > 0 {
		return deployment
	}
	return GuardianInvitationExpiryFallback
}

// GuardianMailRecipient is the guardian a guardian mail addresses.
type GuardianMailRecipient struct {
	FirstName string
	LastName  string
	Email     string
}

// guardianPayload builds the fields every guardian mail shares. linkPath is
// appended to the parents portal origin.
func guardianPayload(recipient GuardianMailRecipient, origin, linkPath, schoolName string) map[string]any {
	origin = MailOrigin(origin)
	return map[string]any{
		GuardianPayloadRecipientEmail: strings.TrimSpace(recipient.Email),
		GuardianPayloadFirstName:      strings.TrimSpace(recipient.FirstName),
		GuardianPayloadLastName:       strings.TrimSpace(recipient.LastName),
		GuardianPayloadInvitationURL:  origin + linkPath,
		GuardianPayloadLogoURL:        origin + motoLogoPath,
		GuardianPayloadSchoolName:     schoolName,
	}
}

// GuardianInvitationPayload is the mail carrying the accept link. expiresAt
// becomes the "valid for N hours" the mail states, floored at one hour so a
// link that is about to expire never advertises zero.
func GuardianInvitationPayload(recipient GuardianMailRecipient, origin, token string, expiresAt time.Time, schoolName string) map[string]any {
	expiryHours := int(time.Until(expiresAt) / time.Hour)
	if expiryHours < 1 {
		expiryHours = 1
	}
	payload := guardianPayload(recipient, origin, "/accept-guardian-invite/"+token, schoolName)
	payload[GuardianPayloadExpiryHours] = expiryHours
	return payload
}

// GuardianExistingAccountPayload is the variant for an address that already
// owns an account (#3320): no registration and no token, just the parents
// portal login and a hint to use the existing credentials. A blank address
// gets no mail: the payload is nil.
func GuardianExistingAccountPayload(recipient GuardianMailRecipient, origin, schoolName string) map[string]any {
	if strings.TrimSpace(recipient.Email) == "" {
		return nil
	}
	payload := guardianPayload(recipient, origin, "/login", schoolName)
	payload[GuardianPayloadExistingAccount] = true
	return payload
}

// errGuardianWelcomePredecessorRequired refuses a welcome that names no
// preceding mail: it could overtake the mail that granted the access.
var errGuardianWelcomePredecessorRequired = errors.New("guardian welcome email: preceding outbox ID is required")

// GuardianWelcomePayload is the welcome that follows a new access (#3534):
// the help article for parents and the Elterninfo, released once the mail in
// outbox row precedingOutboxID was accepted. A blank address gets no mail:
// the payload is nil.
func GuardianWelcomePayload(recipient GuardianMailRecipient, origin, schoolName, helpURL, parentInfoURL string, precedingOutboxID int64) (map[string]any, error) {
	if precedingOutboxID <= 0 {
		return nil, errGuardianWelcomePredecessorRequired
	}
	if strings.TrimSpace(recipient.Email) == "" {
		return nil, nil
	}
	payload := guardianPayload(recipient, origin, "/login", schoolName)
	delete(payload, GuardianPayloadInvitationURL)
	payload[GuardianPayloadHelpURL] = helpURL
	payload[GuardianPayloadParentInfoURL] = parentInfoURL
	payload[GuardianPayloadPrecedingOutboxID] = precedingOutboxID
	return payload, nil
}

// GuardianWelcomeIdempotencyKey names the guardian contact, which belongs to
// one school, so a guardian is welcomed once per school however often they
// are invited or linked to a child.
func GuardianWelcomeIdempotencyKey(guardianProfileID int64) string {
	return fmt.Sprintf("guardian_welcome:%d", guardianProfileID)
}

// GuardianInvitationMail renders a queued guardian invitation and returns
// the address it goes to.
func GuardianInvitationMail(payload map[string]any) (Mail, string, error) {
	recipient, _ := payload[GuardianPayloadRecipientEmail].(string)
	if recipient == "" {
		return Mail{}, "", fmt.Errorf("guardian invitation payload missing recipient_email")
	}
	invitationURL, _ := payload[GuardianPayloadInvitationURL].(string)
	if invitationURL == "" {
		return Mail{}, "", fmt.Errorf("guardian invitation payload missing invitation_url")
	}
	firstName, _ := payload[GuardianPayloadFirstName].(string)
	lastName, _ := payload[GuardianPayloadLastName].(string)
	logoURL, _ := payload[GuardianPayloadLogoURL].(string)
	schoolName, _ := payload[GuardianPayloadSchoolName].(string)
	expiryHours := payloadInt(payload, GuardianPayloadExpiryHours)
	if expiryHours <= 0 {
		expiryHours = int(GuardianInvitationExpiryFallback / time.Hour)
	}
	existingAccount, _ := payload[GuardianPayloadExistingAccount].(bool)
	subject := "Einladung zum Eltern-Portal"
	if existingAccount {
		subject = "Ihr Zugang zum Eltern-Portal"
	}
	return Mail{
		Subject:  schoolSubject(subject, schoolName),
		Template: "guardian-invitation.html",
		Content: map[string]any{
			"InvitationURL":   invitationURL,
			"FirstName":       firstName,
			"LastName":        lastName,
			"ExpiryHours":     expiryHours,
			"LogoURL":         logoURL,
			"SchoolName":      schoolName,
			"ExistingAccount": existingAccount,
		},
	}, recipient, nil
}

// GuardianWelcomeMail renders a queued guardian welcome and returns the
// address it goes to.
func GuardianWelcomeMail(payload map[string]any) (Mail, string, error) {
	recipient, _ := payload[GuardianPayloadRecipientEmail].(string)
	if recipient == "" {
		return Mail{}, "", fmt.Errorf("guardian welcome payload missing recipient_email")
	}
	helpURL, _ := payload[GuardianPayloadHelpURL].(string)
	if helpURL == "" {
		return Mail{}, "", fmt.Errorf("guardian welcome payload missing help_url")
	}
	parentInfoURL, _ := payload[GuardianPayloadParentInfoURL].(string)
	firstName, _ := payload[GuardianPayloadFirstName].(string)
	lastName, _ := payload[GuardianPayloadLastName].(string)
	logoURL, _ := payload[GuardianPayloadLogoURL].(string)
	schoolName, _ := payload[GuardianPayloadSchoolName].(string)
	return Mail{
		Subject:  schoolSubject("Willkommen bei moto", schoolName),
		Template: "guardian-welcome.html",
		Content: map[string]any{
			"HelpURL":       helpURL,
			"ParentInfoURL": parentInfoURL,
			"FirstName":     firstName,
			"LastName":      lastName,
			"LogoURL":       logoURL,
			"SchoolName":    schoolName,
		},
	}, recipient, nil
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

// GuardianWelcomePrecedingOutboxID is the outbox row of the mail that granted
// the access the welcome follows.
func GuardianWelcomePrecedingOutboxID(payload map[string]any) (int64, error) {
	id, ok := payloadInt64(payload, GuardianPayloadPrecedingOutboxID)
	if !ok || id <= 0 {
		return 0, fmt.Errorf("guardian welcome payload missing %s", GuardianPayloadPrecedingOutboxID)
	}
	return id, nil
}

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

// payloadInt64 reads an integer the payload stored; a JSON round trip turns
// it into a float64, which counts only when it is whole.
func payloadInt64(payload map[string]any, key string) (int64, bool) {
	switch value := payload[key].(type) {
	case int:
		return int64(value), true
	case int64:
		return value, true
	case float64:
		return int64(value), value == float64(int64(value))
	default:
		return 0, false
	}
}

// payloadInt reads a count the payload stored, truncating a float64 from a
// JSON round trip; anything else reads as zero.
func payloadInt(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
