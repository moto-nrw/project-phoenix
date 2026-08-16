package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/localization"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/emailbranding"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// parentMessageEmailPreviewRunes bounds the quoted preview. The mail is a
// pointer into the portal, not a copy of the conversation: the full text stays
// behind the parent login, where it belongs.
const parentMessageEmailPreviewRunes = 160

const (
	parentMessageEmailTemplate = "parent-message-notification.html"
	parentMessageEmailType     = "parent_message"
)

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
// Everything here reads inside the request's tenant transaction (RLS-scoped);
// only the dispatch is deferred to after the commit, so a rolled-back reply
// never announces itself. A failure only logs: e-mail must never block a reply.
func (s *Service) notifyGuardianEmail(ctx context.Context, thread *usersModels.ParentMessageThread, body string) {
	if thread == nil || thread.GuardianAccountID <= 0 {
		return
	}
	if s.Dispatcher == nil || s.GuardianProfiles == nil {
		return
	}
	if s.ParentsURL == "" {
		s.Logger.Warn("messaging: guardian e-mail skipped, no parents portal URL configured",
			slog.Int64("thread_id", thread.ID),
		)
		return
	}
	if !s.guardianAcceptsMessageMail(ctx, thread) {
		return
	}

	profile, err := s.GuardianProfiles.FindByAccountID(ctx, thread.GuardianAccountID)
	if err != nil {
		s.Logger.Warn("messaging: load guardian profile for e-mail failed",
			slog.Int64("thread_id", thread.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if profile == nil || profile.Email == nil {
		return
	}
	recipient := strings.TrimSpace(*profile.Email)
	if recipient == "" {
		return
	}

	schoolName, logoURL := s.resolveSchoolBrand(ctx, thread.TenantID)
	childName := s.resolveChildName(ctx, thread.ID)
	locale := localization.DefaultLocale()
	if profile.PortalLocale != nil {
		locale = localization.NormalizeLocale(*profile.PortalLocale)
	}
	copy := messageEmailCopy(locale, profile.FirstName, profile.LastName, schoolName, childName)
	message := email.Message{
		From:     s.DefaultFrom,
		To:       email.NewEmail(strings.TrimSpace(profile.FirstName+" "+profile.LastName), recipient),
		Subject:  copy.Subject,
		Template: parentMessageEmailTemplate,
		Content: map[string]any{
			"Subject":            copy.Subject,
			"GuardianFirstName":  profile.FirstName,
			"GuardianLastName":   profile.LastName,
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
			"ChildName":          childName,
			"Preview":            messagePreview(body),
			"MessagesURL":        s.ParentsURL + "/messages",
			"LogoURL":            logoURL,
			"MotoLogoURL":        emailbranding.MotoLogoURL(s.ParentsURL),
		},
	}

	dispatcher := s.Dispatcher
	request := email.DeliveryRequest{
		Message: message,
		Metadata: email.DeliveryMetadata{
			Type:        parentMessageEmailType,
			ReferenceID: thread.ID,
			Recipient:   recipient,
		},
	}
	tenant.RegisterAfterCommit(ctx, func() {
		dispatcher.Dispatch(context.Background(), request)
	})
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

// resolveChildName returns the child the conversation is about, so the mail says
// which of several children it concerns. Never logged: the name stays in the
// mail body (GDPR — no student names in logs at Info level or above).
func (s *Service) resolveChildName(ctx context.Context, threadID int64) string {
	header, err := s.ReadRepo.FindThreadHeader(ctx, threadID)
	if err != nil || header == nil {
		return ""
	}
	return header.StudentName
}

// messagePreview collapses the message to one line and cuts it to a readable
// taste, ending with an ellipsis when something was left out.
func messagePreview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(preview) <= parentMessageEmailPreviewRunes {
		return preview
	}
	runes := []rune(preview)
	return strings.TrimRight(string(runes[:parentMessageEmailPreviewRunes]), " ") + "…"
}
