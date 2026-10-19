package owners

import (
	"context"
	"errors"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// Payload fields of the queued caterer invitation.
const (
	catererPayloadRecipient   = "recipient_email"
	catererPayloadCatererName = "caterer_name"
	catererPayloadSchoolName  = "school_name"
	catererPayloadURL         = "invitation_url"
	catererPayloadLogoURL     = "logo_url"
	catererPayloadExpiryDays  = "expiry_days"
)

// CatererInvitationDelivery queues a school's invitation to its caterer in
// the outbox (ADR 0074). The link points at the caterer portal and exists
// only in the mail.
func CatererInvitationDelivery(schools schoolFinder, outbox MailResultOutbox, catererURL string) (identityPorts.CatererInvitationDelivery, error) {
	if schools == nil || outbox == nil {
		return nil, errors.New("identity access composition: the caterer invitation needs the school directory and the e-mail outbox")
	}
	return catererInvitationDelivery{schools: schools, outbox: outbox, catererURL: catererURL}, nil
}

type catererInvitationDelivery struct {
	schools    schoolFinder
	outbox     MailResultOutbox
	catererURL string
}

func (d catererInvitationDelivery) EnqueueCatererInvitation(ctx context.Context, invitationID, schoolID int64, recipient, catererName, token string, expiresAt time.Time) error {
	var schoolName string
	if school, found, err := LookupSchool(ctx, d.schools, schoolID); err == nil && found && !school.IsDeleted() {
		schoolName = school.Name
	}
	mail := identityaccess.CatererInvitationMailOf(recipient, catererName, schoolName, d.catererURL, schoolID, token, expiresAt)
	_, err := d.outbox.EnqueueOutboxWithResult(ctx, deliveryCompose.MailEnqueueRequest{
		Kind: identityaccess.MailKindCatererInvitation,
		Payload: map[string]any{
			catererPayloadRecipient: mail.Recipient, catererPayloadCatererName: mail.CatererName,
			catererPayloadSchoolName: mail.SchoolName, catererPayloadURL: mail.InvitationURL,
			catererPayloadLogoURL: mail.LogoURL, catererPayloadExpiryDays: mail.ExpiryDays,
		},
		RelatedEntityType: identityaccess.MailRelatedCatererInvitation,
		RelatedEntityID:   invitationID,
	})
	return err
}

// NewCatererInvitationRenderer renders the queued caterer invitation.
func NewCatererInvitationRenderer(defaultFrom email.Email) func(context.Context, map[string]any) (*email.Message, error) {
	return func(_ context.Context, payload map[string]any) (*email.Message, error) {
		text := func(key string) string {
			value, _ := payload[key].(string)
			return value
		}
		days, _ := payloadIntField(payload, catererPayloadExpiryDays)
		queued := identityaccess.CatererMail{
			Recipient: text(catererPayloadRecipient), CatererName: text(catererPayloadCatererName),
			SchoolName: text(catererPayloadSchoolName), InvitationURL: text(catererPayloadURL),
			LogoURL: text(catererPayloadLogoURL), ExpiryDays: days,
		}
		mail, err := identityaccess.CatererInvitationMail(queued)
		if err != nil {
			return nil, err
		}
		return identityMessage(defaultFrom, email.NewEmail("", queued.Recipient), mail), nil
	}
}
