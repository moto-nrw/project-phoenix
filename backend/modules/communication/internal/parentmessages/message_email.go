package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/localization"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

const (
	parentMessageEmailTemplate = "parent-message-notification.html"
	parentMessageEmailType     = "parent_message"
	messagePayloadRecipient    = "recipient_email"
	messagePayloadFirstName    = "first_name"
	messagePayloadLastName     = "last_name"
	messagePayloadLocale       = "locale"
	messagePayloadSchoolName   = "school_name"
	messagePayloadMessagesURL  = "messages_url"
	messagePayloadLogoURL      = "logo_url"
	messagePayloadMotoLogoURL  = "moto_logo_url"
)

type ParentMessageRendererConfig struct {
	DefaultFrom email.Email
}

type parentMessageEmailCopy struct {
	Subject            string
	Kicker             string
	Greeting           string
	Intro              string
	Reply              string
	FallbackHint       string
	PreferenceHint     string
	FooterText         string
	PoweredByLabel     string
	SchoolLogoAlt      string
	DefaultBrandKicker string
	DefaultSchoolName  string
}

// GuardianProfileFinder resolves the guardian's e-mail address and name inside
// the current tenant transaction.
type GuardianProfileFinder interface {
	FindByAccountID(ctx context.Context, accountID int64) (*usersModels.GuardianProfile, error)
}

// SchoolFinder resolves the school that sends the mail.
type SchoolFinder interface {
	FindByID(ctx context.Context, id int64) (*platformModels.School, error)
}

// LoginImageResolver resolves the school's logo for the mail header. Optional:
// without it the header renders the neutral OGS badge.
type LoginImageResolver interface {
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)
}

// notifyGuardianEmail tells the guardian by e-mail that the OGS wrote to them
// (#2307). Push reaches only guardians who set it up, and on iPhone only after
// the app sits on the Home Screen; a school reported an Elterninfo that reached
// nobody. The e-mail is the fallback that closes that gap.
//
// Consent follows the e-mail rule, not the push rule: FilterNotOptedOut drops
// only guardians who explicitly declined this notification and keeps everyone
// who never decided, because the school already writes to this address.
//
// Everything here runs in the request's tenant transaction. The message and
// durable outbox row therefore commit or roll back together.
func (s *Service) notifyGuardianEmail(ctx context.Context, thread *usersModels.ParentMessageThread, messageID int64) error {
	if thread == nil || thread.GuardianAccountID <= 0 {
		return nil
	}
	if s.Outbox == nil || s.GuardianProfiles == nil {
		return nil
	}
	if s.ParentsURL == "" {
		s.Logger.Warn("messaging: guardian e-mail skipped, no parents portal URL configured",
			slog.Int64("thread_id", thread.ID),
		)
		return nil
	}
	if !s.guardianAcceptsMessageMail(ctx, thread) {
		return nil
	}

	profile, err := s.GuardianProfiles.FindByAccountID(ctx, thread.GuardianAccountID)
	if err != nil {
		s.Logger.Warn("messaging: load guardian profile for e-mail failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	if profile == nil || profile.Email == nil {
		return nil
	}
	recipient := strings.TrimSpace(*profile.Email)
	if recipient == "" {
		return nil
	}

	schoolName, logoURL := s.resolveSchoolBrand(ctx, thread.TenantID)
	locale := localization.DefaultLocale()
	if profile.PortalLocale != nil {
		locale = localization.NormalizeLocale(*profile.PortalLocale)
	}
	request := platformModels.OutboxEnqueueRequest{
		Kind:              platformModels.EmailKindParentMessage,
		RelatedEntityType: platformModels.EmailRelatedTypeParentMessage,
		RelatedEntityID:   messageID,
		IdempotencyKey:    parentMessageEmailType + ":" + strconv.FormatInt(messageID, 10),
		Payload: map[string]any{
			messagePayloadRecipient:   recipient,
			messagePayloadFirstName:   profile.FirstName,
			messagePayloadLastName:    profile.LastName,
			messagePayloadLocale:      locale,
			messagePayloadSchoolName:  schoolName,
			messagePayloadMessagesURL: strings.TrimRight(s.ParentsURL, "/") + "/messages/" + strconv.FormatInt(thread.StudentID, 10),
			messagePayloadLogoURL:     logoURL,
			messagePayloadMotoLogoURL: emailbranding.MotoLogoURL(s.ParentsURL),
		},
	}
	if err := s.Outbox.EnqueueOutbox(ctx, request); err != nil {
		return fmt.Errorf("messaging: enqueue guardian message e-mail: %w", err)
	}
	return nil
}

func NewParentMessageRenderer(cfg ParentMessageRendererConfig) func(context.Context, *platformModels.EmailOutbox) (*email.Message, error) {
	return func(_ context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
		recipient, _ := row.Payload[messagePayloadRecipient].(string)
		if strings.TrimSpace(recipient) == "" {
			return nil, fmt.Errorf("%s payload missing recipient_email", row.Kind)
		}
		first, _ := row.Payload[messagePayloadFirstName].(string)
		last, _ := row.Payload[messagePayloadLastName].(string)
		locale, _ := row.Payload[messagePayloadLocale].(string)
		schoolName, _ := row.Payload[messagePayloadSchoolName].(string)
		messagesURL, _ := row.Payload[messagePayloadMessagesURL].(string)
		logoURL, _ := row.Payload[messagePayloadLogoURL].(string)
		motoLogoURL, _ := row.Payload[messagePayloadMotoLogoURL].(string)
		if messagesURL == "" {
			return nil, fmt.Errorf("%s payload missing messages_url", row.Kind)
		}
		copy := messageEmailCopy(locale, first, last, schoolName, "")
		return &email.Message{
			From:     cfg.DefaultFrom,
			To:       email.NewEmail(strings.TrimSpace(first+" "+last), recipient),
			Subject:  copy.Subject,
			Template: parentMessageEmailTemplate,
			Content: map[string]any{
				"Subject":            copy.Subject,
				"GuardianFirstName":  first,
				"GuardianLastName":   last,
				"SchoolName":         schoolName,
				"BrandKicker":        copy.Kicker,
				"Greeting":           copy.Greeting,
				"IntroText":          copy.Intro,
				"ReplyLabel":         copy.Reply,
				"FallbackHint":       copy.FallbackHint,
				"PreferenceHint":     copy.PreferenceHint,
				"FooterText":         copy.FooterText,
				"PoweredByLabel":     copy.PoweredByLabel,
				"SchoolLogoAlt":      copy.SchoolLogoAlt,
				"DefaultBrandKicker": copy.DefaultBrandKicker,
				"DefaultSchoolName":  copy.DefaultSchoolName,
				"MessagesURL":        messagesURL,
				"LogoURL":            logoURL,
				"MotoLogoURL":        motoLogoURL,
			},
		}, nil
	}
}

func messageEmailCopy(locale, firstName, lastName, schoolName, childName string) parentMessageEmailCopy {
	name := strings.TrimSpace(firstName + " " + lastName)
	school := strings.TrimSpace(schoolName)
	child := strings.TrimSpace(childName)
	switch localization.NormalizeLocale(locale) {
	case "en":
		return parentMessageEmailCopy{
			Subject: "New message from the OGS", Kicker: "New message",
			Greeting: localizedGreeting("Hello", name, ","),
			Intro:    localizedMessageIntro("The OGS", school, "sent you a message", "about", child),
			Reply:    "Reply", FallbackHint: "If the button does not work, copy this link:",
			PreferenceHint: "You are receiving this email because you are registered as a guardian in moto. You can choose which notifications you receive in the app under Notifications.",
			FooterText:     localizedEmailFooter("This email was sent on behalf of", school, "This email was sent automatically."),
			PoweredByLabel: "Powered by", SchoolLogoAlt: "School logo",
			DefaultBrandKicker: "Parent portal", DefaultSchoolName: "Your OGS",
		}
	case "ru":
		return parentMessageEmailCopy{
			Subject: "Новое сообщение от продлёнки", Kicker: "Новое сообщение",
			Greeting: localizedGreeting("Здравствуйте", name, "!"),
			Intro:    localizedMessageIntro("Продлёнка", school, "отправила вам сообщение", "о ребёнке", child),
			Reply:    "Ответить", FallbackHint: "Если кнопка не работает, скопируйте эту ссылку:",
			PreferenceHint: "Вы получили это письмо, потому что указаны в moto как законный представитель. В разделе «Уведомления» приложения можно выбрать, о чём мы будем вас информировать.",
			FooterText:     localizedEmailFooter("Это письмо отправлено от имени", school, "Это письмо отправлено автоматически."),
			PoweredByLabel: "При поддержке", SchoolLogoAlt: "Логотип школы",
			DefaultBrandKicker: "Родительский портал", DefaultSchoolName: "Ваша продлёнка",
		}
	case "sq":
		return parentMessageEmailCopy{
			Subject: "Mesazh i ri nga OGS-ja", Kicker: "Mesazh i ri",
			Greeting: localizedGreeting("Përshëndetje", name, ","),
			Intro:    localizedMessageIntro("OGS-ja", school, "ju ka dërguar një mesazh", "për", child),
			Reply:    "Përgjigju", FallbackHint: "Nëse butoni nuk funksionon, kopjoni këtë lidhje:",
			PreferenceHint: "Po e merrni këtë email sepse jeni regjistruar si kujdestar në moto. Te Njoftimet në aplikacion mund të zgjidhni për çfarë dëshironi të njoftoheni.",
			FooterText:     localizedEmailFooter("Ky email u dërgua në emër të", school, "Ky email u dërgua automatikisht."),
			PoweredByLabel: "Mundësuar nga", SchoolLogoAlt: "Logoja e shkollës",
			DefaultBrandKicker: "Portali i prindërve", DefaultSchoolName: "OGS-ja juaj",
		}
	default:
		return parentMessageEmailCopy{
			Subject: "Neue Nachricht von der OGS", Kicker: "Neue Nachricht",
			Greeting: localizedGreeting("Guten Tag", name, ","),
			Intro:    localizedMessageIntro("Die OGS", school, "hat Ihnen eine Nachricht geschrieben", "zu", child),
			Reply:    "Antworten", FallbackHint: "Falls der Button nicht funktioniert, kopieren Sie bitte diesen Link:",
			PreferenceHint: "Sie erhalten diese E-Mail, weil Sie in moto als sorgeberechtigte Person hinterlegt sind. In der App können Sie unter „Benachrichtigungen“ einstellen, worüber wir Sie informieren.",
			FooterText:     localizedEmailFooter("Diese E-Mail wurde im Auftrag von", school, "Diese E-Mail wurde automatisch versendet."),
			PoweredByLabel: "Unterstützt von", SchoolLogoAlt: "Logo der Schule",
			DefaultBrandKicker: "Elternportal", DefaultSchoolName: "Ihre OGS",
		}
	}
}

func localizedEmailFooter(prefix, school, fallback string) string {
	if school == "" {
		return fallback
	}
	return prefix + " " + school + "."
}

func localizedGreeting(prefix, name, suffix string) string {
	if name == "" {
		return prefix + suffix
	}
	return prefix + " " + name + suffix
}

func localizedMessageIntro(sender, school, action, childConnector, child string) string {
	if school != "" {
		sender += " " + school
	}
	if child != "" {
		return fmt.Sprintf("%s %s %s %s:", sender, action, childConnector, child)
	}
	return fmt.Sprintf("%s %s:", sender, action)
}

// guardianAcceptsMessageMail reports whether the guardian has NOT declined the
// new-message notification. Without a preference service nobody is filtered:
// the address is one the school already writes to.
func (s *Service) guardianAcceptsMessageMail(ctx context.Context, thread *usersModels.ParentMessageThread) bool {
	if s.Preferences == nil {
		return true
	}
	remaining, err := s.Preferences.FilterNotOptedOut(ctx, notifications.TypeParentMessage, []int64{thread.GuardianAccountID})
	if err != nil {
		// A consent lookup that did not answer is not a yes.
		s.Logger.Warn("messaging: guardian e-mail consent check failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return len(remaining) > 0
}

// resolveSchoolBrand returns the school name and logo URL for the mail chrome.
// Both are cosmetic: a lookup failure leaves the template's neutral fallbacks.
func (s *Service) resolveSchoolBrand(ctx context.Context, tenantID int64) (string, string) {
	if s.Schools == nil || tenantID <= 0 {
		return "", ""
	}
	school, err := s.Schools.FindByID(ctx, tenantID)
	if err != nil || school == nil {
		return "", ""
	}
	logoURL := ""
	if s.LoginImages != nil {
		if raw, imgErr := s.LoginImages.GetLoginImageURL(ctx, tenantID); imgErr == nil {
			logoURL = emailbranding.SchoolLogoURL(s.ParentsURL, raw)
		}
	}
	return school.Name, logoURL
}
