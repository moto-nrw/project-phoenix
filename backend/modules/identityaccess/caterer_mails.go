package identityaccess

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Delivery outbox kind of the caterer invitation (ADR 0077) and the entity
// it relates to. The composition root enqueues the mail and registers its
// renderer under the same kind.
const (
	MailKindCatererInvitation    = "caterer_invitation"
	MailRelatedCatererInvitation = "caterer_invitation"
)

// CatererMail is the invitation mail as the outbox stores it. Identity
// keeps only the link's hash; the outbox keeps the mail until its delivery
// retention ends. No school-facing read returns it, so only the owner of the
// address can accept.
type CatererMail struct {
	Recipient     string
	CatererName   string
	SchoolName    string
	InvitationURL string
	LogoURL       string
	ExpiryDays    int
}

// CatererInvitationMailOf builds the mail of a school's invitation. The link
// names the school, so the caterer portal opens only that school.
func CatererInvitationMailOf(recipient, catererName, schoolName, origin string, schoolID int64, token string, expiresAt time.Time) CatererMail {
	origin = MailOrigin(origin)
	return CatererMail{
		Recipient:     strings.TrimSpace(recipient),
		CatererName:   strings.TrimSpace(catererName),
		SchoolName:    strings.TrimSpace(schoolName),
		InvitationURL: fmt.Sprintf("%s/einladung/%d/%s", origin, schoolID, token),
		LogoURL:       origin + motoLogoPath,
		ExpiryDays:    max(int(math.Ceil(time.Until(expiresAt).Hours()/24)), 1),
	}
}

// CatererInvitationMail renders the stored invitation.
func CatererInvitationMail(mail CatererMail) (Mail, error) {
	if mail.Recipient == "" {
		return Mail{}, errors.New("caterer invitation payload missing recipient")
	}
	if mail.InvitationURL == "" {
		return Mail{}, errors.New("caterer invitation payload missing invitation_url")
	}
	return Mail{
		Subject:  schoolSubject("Einladung zum Mittagessen bei moto", mail.SchoolName),
		Template: "caterer-invitation.html",
		Content: map[string]any{
			"InvitationURL": mail.InvitationURL,
			"CatererName":   mail.CatererName,
			"SchoolName":    mail.SchoolName,
			"ExpiryDays":    mail.ExpiryDays,
			"LogoURL":       mail.LogoURL,
		},
	}, nil
}
