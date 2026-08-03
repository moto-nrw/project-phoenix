package calendar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GuardianChildAccessChecker answers, at the moment a queued mail is sent, which
// of its addressed guardian accounts still hold parent_portal.access for at
// least one of the children the appointment concerns.
type GuardianChildAccessChecker interface {
	FilterAccountsWithStudentAccess(ctx context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error)
}

// EmailConfig is the static config for the appointment e-mail renderer.
type EmailConfig struct {
	DefaultFrom email.Email

	// DB and Guardians re-check the authorization scope a row carries before it
	// is rendered. Both are required for rows that carry one; rows without a
	// scope (queued before this existed) render as they always did.
	DB        *bun.DB
	Guardians GuardianChildAccessChecker
}

// NewAppointmentRenderer renders any appointment outbox row (published /
// updated / cancelled / reminder) into an e-mail. One renderer handles all four
// kinds — they share a template and differ only in the intro copy + subject.
// Registered in the email template registry for each appointment kind and
// drained by the outbox worker.
func NewAppointmentRenderer(cfg EmailConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(ctx context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		if err := cfg.ensureGuardianStillPermitted(ctx, row); err != nil {
			return nil, err
		}
		recipient, _ := row.Payload[apptPayloadRecipient].(string)
		if recipient == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
		}
		title, _ := row.Payload[apptPayloadTitle].(string)
		when, _ := row.Payload[apptPayloadWhen].(string)
		location, _ := row.Payload[apptPayloadLocation].(string)
		schoolName, _ := row.Payload[apptPayloadSchoolName].(string)
		first, _ := row.Payload[apptPayloadFirstName].(string)
		last, _ := row.Payload[apptPayloadLastName].(string)
		portalURL, _ := row.Payload[apptPayloadPortalURL].(string)
		logoURL, _ := row.Payload[apptPayloadLogoURL].(string)
		motoLogoURL, _ := row.Payload[apptPayloadMotoLogoURL].(string)

		kicker, subjectVerb, intro := appointmentCopy(row.Kind, schoolName)
		subject := fmt.Sprintf("%s: %s", subjectVerb, title)
		if schoolName != "" {
			subject = fmt.Sprintf("%s – %s: %s", schoolName, subjectVerb, title)
		}

		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail("", recipient),
			Subject:  subject,
			Template: "appointment-notification.html",
			Content: map[string]any{
				"Title":             title,
				"BrandKicker":       kicker,
				"IntroText":         intro,
				"WhenText":          when,
				"Location":          location,
				"SchoolName":        schoolName,
				"GuardianFirstName": first,
				"GuardianLastName":  last,
				"PortalURL":         portalURL,
				"LogoURL":           logoURL,
				"MotoLogoURL":       motoLogoURL,
			},
		}, nil
	}
}

// ensureGuardianStillPermitted asks the child-access question again at the
// moment the mail is sent, and refuses to render when the answer changed.
//
// The recipient list of an appointment mail is resolved when it is queued, but
// the mail leaves later — a reminder waits for the school's whole lead time,
// 24 hours by default. An appointment mail names a title, a time and a place;
// once a guardian's parent_portal.access for the child that put them on the
// invitation is revoked, they are no longer someone this school tells those
// things to, and cancelling every queued row from every revocation path would
// have to catch each of those paths to be worth anything. Asking here is the
// same recheck the push channels do immediately before delivery.
//
// Rows without a scope have nothing to re-check: they predate this or address a
// recipient with no portal account, and they render as they always did.
func (cfg EmailConfig) ensureGuardianStillPermitted(ctx context.Context, row *platformModels.EmailOutbox) error {
	accountID := payloadInt64(row.Payload[apptPayloadGuardianAccountID])
	studentIDs := payloadInt64Slice(row.Payload[apptPayloadStudentIDs])
	if accountID <= 0 || len(studentIDs) == 0 {
		return nil
	}
	tenantID := row.GetTenantID()
	if cfg.DB == nil || cfg.Guardians == nil || tenantID <= 0 {
		// The row asked to be re-checked and nothing here can answer. Sending it
		// anyway would defeat the point of stamping the scope in the first place.
		return fmt.Errorf("%w: cannot re-check guardian child access for %s",
			platformService.ErrRenderCancelled, row.Kind)
	}

	var permitted []int64
	if err := tenant.WithTenantTx(ctx, cfg.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		allowed, err := cfg.Guardians.FilterAccountsWithStudentAccess(
			txCtx, []int64{accountID}, studentIDs, tenantID, authorize.GuardianPermissionPortalAccess)
		if err != nil {
			return err
		}
		permitted = allowed
		return nil
	}); err != nil {
		// A lookup failure is not an answer — retry it rather than dropping a mail
		// the guardian may well still be entitled to.
		return fmt.Errorf("re-check guardian child access: %w", err)
	}
	if len(permitted) == 0 {
		return fmt.Errorf("%w: guardian no longer has access to the children of this appointment",
			platformService.ErrRenderCancelled)
	}
	return nil
}

// payloadInt64 reads an ID out of a jsonb payload. A row that round-tripped
// through the database arrives as float64 (or json.Number under a decoder that
// asked for it); one that never left the process keeps its Go type.
func payloadInt64(raw any) int64 {
	switch value := raw.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func payloadInt64Slice(raw any) []int64 {
	if direct, ok := raw.([]int64); ok {
		return direct
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if id := payloadInt64(item); id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// appointmentCopy returns the German header kicker, subject verb and intro
// sentence for each appointment e-mail kind.
//
// The intro is a COMPLETE sentence that already names the sender, and it
// continues the "Guten Tag Frau Schneider," greeting — hence lower case. The
// template used to glue a "die {Schule}: " prefix in front of a sentence that
// did not expect one, which produced "die OGS Musterschule: ein Termin wurde
// abgesagt." Naming the school inside the sentence is the only way the two
// halves can agree grammatically.
func appointmentCopy(kind, schoolName string) (kicker, subjectVerb, intro string) {
	sender := "die " + schoolName
	known := schoolName != ""

	switch kind {
	case platformModels.EmailKindAppointmentUpdated:
		if known {
			return "Termin geändert", "Termin geändert", sender + " hat einen Termin geändert."
		}
		return "Termin geändert", "Termin geändert", "ein Termin wurde geändert."
	case platformModels.EmailKindAppointmentCancelled:
		if known {
			return "Termin abgesagt", "Termin abgesagt", sender + " hat einen Termin abgesagt."
		}
		return "Termin abgesagt", "Termin abgesagt", "ein Termin wurde abgesagt."
	case platformModels.EmailKindAppointmentReminder:
		if known {
			return "Terminerinnerung", "Erinnerung", sender + " erinnert Sie an einen bevorstehenden Termin."
		}
		return "Terminerinnerung", "Erinnerung", "wir erinnern Sie an einen bevorstehenden Termin."
	default:
		if known {
			return "Neuer Termin", "Neuer Termin", sender + " hat einen neuen Termin für Sie eingetragen."
		}
		return "Neuer Termin", "Neuer Termin", "es gibt einen neuen Termin für Sie."
	}
}
