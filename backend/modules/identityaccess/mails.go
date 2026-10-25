package identityaccess

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// The mails of the Identity & Access flows (#4082): what each one says, which
// template renders it and where its links point. The composition root only
// addresses a Mail, attaches the sender and hands it to Delivery.

// Mail is the content of one Identity & Access mail: the subject, the
// template and the values the template renders.
type Mail struct {
	Subject  string
	Template string
	Content  any
}

// localPortalOrigin is where a mail link points when the portal URL is not
// configured: the local frontend.
const localPortalOrigin = "http://localhost:3000"

// motoLogoPath is the moto logo every mail embeds, served by the frontend.
const motoLogoPath = "/images/moto-logo-mit-schriftzug.png"

// MailOrigin trims the configured portal URL; an unset one points at the
// local frontend, like the invitation links.
func MailOrigin(origin string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		return localPortalOrigin
	}
	return origin
}

// schoolSubject names the school after the subject when it is known.
func schoolSubject(subject, schoolName string) string {
	if schoolName == "" {
		return subject
	}
	return fmt.Sprintf("%s – %s", subject, schoolName)
}

// trimmedLogoURL is the logo at an origin that may end in a slash.
func trimmedLogoURL(origin string) string {
	return strings.TrimRight(origin, "/") + motoLogoPath
}

// --- the school invitation ---------------------------------------------------

// systemRoleDisplayNames maps the system role names to the German display
// names the invitation mail shows. They match SYSTEM_ROLE_LABELS in
// frontend/src/lib/role-labels.ts, the one name per role the school sees
// everywhere (#4574).
var systemRoleDisplayNames = map[string]string{
	"admin":           "Verwaltung",
	"user":            "Betreuung",
	"guest":           "Gast",
	"guardian":        "Eltern",
	"lehrkraft":       "Lehrkraft",
	"ag-leitung":      "AG-Leitung",
	"schulverwaltung": "Schulverwaltung",
}

// RoleDisplayName is the German name the invitation mail shows for a role;
// a school's own role keeps its name.
func RoleDisplayName(roleName string) string {
	if translated, ok := systemRoleDisplayNames[strings.ToLower(roleName)]; ok {
		return translated
	}
	return roleName
}

// StaffWelcomeDelay keeps the welcome behind the invitation while preserving
// that delay in the durable outbox.
const StaffWelcomeDelay = 30 * time.Second

// SchoolInvitationOrigin is the portal an invitee accepts on: the staff
// portal, or the local frontend when it is not configured. A school-portal
// role accepts on the school portal (#2207): the link must land where its
// login lives, otherwise the Lehrkraft sets a password in the staff portal
// and then cannot use it there.
func SchoolInvitationOrigin(portal InvitationPortal, staffURL, schoolURL string) string {
	origin := staffURL
	if origin == "" {
		origin = localPortalOrigin
	}
	if portal == InvitationPortalSchool && schoolURL != "" {
		origin = schoolURL
	}
	return origin
}

// SchoolInvitationMail is the mail carrying the accept link of a school
// invitation, valid for expiry.
func SchoolInvitationMail(invitation SchoolInvitation, schoolName, origin string, expiry time.Duration) Mail {
	return Mail{
		Subject:  schoolSubject("Einladung zu moto", schoolName),
		Template: "invitation.html",
		Content: map[string]any{
			"InvitationURL": fmt.Sprintf("%s/invite?token=%s", origin, invitation.Token),
			"RoleName":      RoleDisplayName(invitation.RoleName),
			"FirstName":     invitation.FirstName,
			"LastName":      invitation.LastName,
			"ExpiryHours":   int(expiry / time.Hour),
			"LogoURL":       origin + motoLogoPath,
			"SchoolName":    schoolName,
		},
	}
}

// --- the staff welcome --------------------------------------------------------

// StaffWelcome is what a queued staff welcome names.
type StaffWelcome struct {
	Recipient  string
	HelpURL    string
	FirstName  string
	LogoURL    string
	SchoolName string
}

// NewStaffWelcome is the welcome that follows a school invitation accepted
// at origin.
func NewStaffWelcome(invitation SchoolInvitation, schoolName, origin, helpURL string) StaffWelcome {
	firstName := ""
	if invitation.FirstName != nil {
		firstName = strings.TrimSpace(*invitation.FirstName)
	}
	return StaffWelcome{
		Recipient: invitation.Email, HelpURL: helpURL, FirstName: firstName,
		LogoURL: origin + motoLogoPath, SchoolName: schoolName,
	}
}

// StaffWelcomeCancelled reports an invitation lookup outcome after which the
// welcome is never sent: the invitation was used, expired, is gone or its
// mail failed for good.
func StaffWelcomeCancelled(err error) bool {
	for _, gone := range []error{ErrInvitationUsed, ErrInvitationExpired, ErrInvitationNotFound, ErrInvitationDeliveryFailed} {
		if errors.Is(err, gone) {
			return true
		}
	}
	return false
}

// StaffWelcomeMail renders a queued staff welcome. replyGoesToSchool, asked
// once the welcome is complete, tells the reader whether answering reaches
// the school rather than moto.
func StaffWelcomeMail(welcome StaffWelcome, replyGoesToSchool func() bool) (Mail, error) {
	if welcome.Recipient == "" {
		return Mail{}, errors.New("staff welcome payload missing recipient_email")
	}
	if welcome.HelpURL == "" {
		return Mail{}, errors.New("staff welcome payload missing help_url")
	}
	return Mail{
		Subject:  schoolSubject("Willkommen bei moto", welcome.SchoolName),
		Template: "staff-welcome.html",
		Content: map[string]any{
			"HelpURL":           welcome.HelpURL,
			"FirstName":         strings.TrimSpace(welcome.FirstName),
			"LogoURL":           welcome.LogoURL,
			"SchoolName":        welcome.SchoolName,
			"ReplyGoesToSchool": replyGoesToSchool(),
		},
	}, nil
}

// --- password reset -----------------------------------------------------------

// PasswordResetMail is the mail carrying the reset link of a portal at
// origin, valid for expiry.
func PasswordResetMail(link PasswordResetLink, origin string, expiry time.Duration) Mail {
	origin = strings.TrimRight(origin, "/")
	return Mail{
		Subject:  "Passwort zurücksetzen",
		Template: "password-reset.html",
		Content: map[string]any{
			"ResetURL":      fmt.Sprintf("%s/reset-password?token=%s", origin, link.Token),
			"ExpiryMinutes": int(expiry.Minutes()),
			"LogoURL":       origin + motoLogoPath,
		},
	}
}

// --- the second factor --------------------------------------------------------

// MFACodeMailContent is the mail carrying a sign-in code. The template
// carries no link or button on purpose: the user pastes the code into the
// moto login, which kills the "click here to confirm" phishing pattern.
func MFACodeMailContent(message MFACodeMail, origin string) Mail {
	return Mail{
		Subject:  "Ihr moto-Anmeldecode",
		Template: "mfa-email-code.html",
		Content: map[string]any{
			"LogoURL":              trimmedLogoURL(origin),
			"Code":                 message.Code,
			"ExpiryMinutes":        message.ExpiryMinutes,
			"RequestIP":            message.RequestIP,
			"TrustedDeviceEnabled": message.TrustedDeviceEnabled,
			"TrustedDeviceDays":    message.TrustedDeviceDays,
		},
	}
}

// TrustedDeviceAddedMailContent tells the holder that a device was trusted.
func TrustedDeviceAddedMailContent(message TrustedDeviceMail, origin string) Mail {
	return Mail{
		Subject:  "Neues vertrautes Gerät zu Ihrem moto-Konto hinzugefügt",
		Template: "trusted-device-added.html",
		Content: map[string]any{
			"LogoURL":     trimmedLogoURL(origin),
			"DeviceLabel": message.DeviceLabel,
			"IPAddress":   message.RequestIP,
			"AddedAt":     message.AddedAt,
			"TrustedDays": message.TrustedDays,
		},
	}
}

// --- the operator links -------------------------------------------------------

// operatorInviterFallback names an inviter the invitation does not know.
const operatorInviterFallback = "Ein Operator"

// OperatorInvitationMail is the mail carrying an operator invitation link
// on the operator portal at operatorOrigin; origin serves the logo. The
// path is /invite, not /operator/invite: the operator subdomain proxy
// rewrites it internally.
func OperatorInvitationMail(invitation OperatorInvitation, inviterName, origin, operatorOrigin string, expiry time.Duration) Mail {
	if inviterName == "" {
		inviterName = operatorInviterFallback
	}
	return Mail{
		Subject:  "Einladung als Operator zu moto",
		Template: "operator-invitation.html",
		Content: map[string]any{
			"InvitationURL": fmt.Sprintf("%s/invite?token=%s", operatorOrigin, invitation.Token),
			"ExpiryHours":   int(expiry.Hours()),
			"LogoURL":       origin + motoLogoPath,
			"InviterName":   inviterName,
		},
	}
}

// OperatorEmailChangeVerificationMail carries the confirmation link of a new
// operator address. The link goes through /operator/* → operator subdomain
// redirect, which preserves the query. The page strips the token from the
// URL on mount, so no later Referer carries it.
func OperatorEmailChangeVerificationMail(change OperatorEmailChange, origin string, expiry time.Duration) Mail {
	return Mail{
		Subject:  "E-Mail-Adresse bestätigen",
		Template: "operator-email-change-verify.html",
		Content: map[string]any{
			"VerifyURL":     fmt.Sprintf("%s/operator/email-confirm?token=%s", origin, change.Token),
			"ExpiryMinutes": int(expiry.Minutes()),
			"LogoURL":       origin + motoLogoPath,
		},
	}
}

// OperatorEmailChangeRequestedMail tells the operator's current address
// that a change to maskedNewEmail was requested.
func OperatorEmailChangeRequestedMail(operator Operator, maskedNewEmail, origin string) Mail {
	return Mail{
		Subject:  "E-Mail-Änderung angefordert",
		Template: "operator-email-change-notify.html",
		Content: map[string]any{
			"LogoURL":        origin + motoLogoPath,
			"MaskedNewEmail": maskedNewEmail,
			"DisplayName":    operator.DisplayName,
		},
	}
}

// OperatorEmailChangeConfirmedMail tells the old address that the change
// took effect.
func OperatorEmailChangeConfirmedMail(displayName, origin string) Mail {
	return Mail{
		Subject:  "E-Mail-Adresse wurde geändert",
		Template: "operator-email-change-confirmed.html",
		Content: map[string]any{
			"LogoURL":     origin + motoLogoPath,
			"DisplayName": displayName,
		},
	}
}

// --- the Träger-Büro invitation ---------------------------------------------

// TraegerInvitationMail is the mail carrying an invitation into a
// Träger-Büro (#4650). The link goes to the Träger-Portal at traegerOrigin;
// its proxy serves /invite from the Träger routes. origin serves the logo.
func TraegerInvitationMail(invitation TraegerInvitation, traegerName, origin, traegerOrigin string, expiry time.Duration) Mail {
	return Mail{
		Subject:  "Einladung ins Träger-Portal von moto",
		Template: "traeger-invitation.html",
		Content: map[string]any{
			"InvitationURL": fmt.Sprintf("%s/invite?token=%s", strings.TrimRight(traegerOrigin, "/"), invitation.Token),
			"TraegerName":   traegerName,
			"RoleNames":     TraegerRoleLabels(invitation.Roles),
			"ExpiryHours":   int(expiry / time.Hour),
			"LogoURL":       origin + motoLogoPath,
		},
	}
}

// --- the public demo ----------------------------------------------------------

// The demo team receives the lead mail and the prospects' answers.
const (
	DemoContactName    = "moto"
	DemoContactAddress = "kontakt@moto-ogs.de"
)

// The demo templates are the only ones the demo environment sends.
const (
	DemoAccessTemplate = "demo-access.html"
	DemoLeadTemplate   = "demo-lead.html"
)

// PersonName is how the lead mail names the prospect.
func (m DemoAccessMessage) PersonName() string {
	return m.FirstName + " " + m.LastName
}

// DemoAccessLinkMail is the link back into the demo for the prospect.
// Anyone can type a foreign address into the public form, so nothing the
// form carried reaches this mail: no name, no school.
func DemoAccessLinkMail(entryURL, origin string) Mail {
	return Mail{
		Subject:  "Ihr Link zur moto-Demo",
		Template: DemoAccessTemplate,
		Content: map[string]any{
			"EntryURL": entryURL,
			"LogoURL":  trimmedLogoURL(origin),
		},
	}
}

// DemoLeadMail tells the demo team about a new access.
func DemoLeadMail(access DemoAccessMessage, origin string) Mail {
	return Mail{
		Subject:  "Neuer Demo-Zugang: " + access.SchoolName,
		Template: DemoLeadTemplate,
		Content: map[string]any{
			"PersonName":   access.PersonName(),
			"OGSName":      access.SchoolName,
			"Email":        access.Email,
			"Source":       access.Source,
			"ContactOptIn": access.ContactOptIn,
			"AccessID":     access.AccessID,
			"LogoURL":      trimmedLogoURL(origin),
		},
	}
}
