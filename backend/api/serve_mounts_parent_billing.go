package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	parentBillingHTTP "github.com/moto-nrw/project-phoenix/modules/parentbilling/http"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// mountParentBillingRoutes serves the Elternabrechnung (ADR 0088, #4664, ADR
// 0115): the contribution rules of a Träger at /traeger/contribution-rules
// behind the right "Abrechnung", the contribution and confirmation of a
// holiday registration at /traeger/holiday-contributions behind the right
// "Anmeldung verwalten", and the public contribution of the registration
// form at /traeger/holiday-quotes, and the billing runs and number ranges
// at /traeger/billing-runs and /traeger/number-ranges behind the right
// "Abrechnung" (#4668, ADR 0119), with the confirmation letters of a
// released run in the layout of the Träger (#4669, ADR 0121), its overview
// for the Kämmerei (#4672, ADR 0122), and the mail of the letters as mails
// of the Träger on Delivery's outbox and their print in one file (#4670,
// ADR 0123).
func mountParentBillingRoutes(root chi.Router, organizations HolidayOrganizations, settings TraegerBrandingSettings, delivery deliveryRuntime.Capability, healthData enrollmentOwner.HolidayHealthCipher, logger *slog.Logger) {
	rules, holiday := NewParentBillingHTTP(organizations, delivery, healthData, logger)
	root.Mount("/traeger/contribution-rules", rules.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-contributions", holiday.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-quotes", holiday.QuoteRouter())
	runs := NewBillingRunHTTP(settings, delivery, healthData)
	root.Mount("/traeger/billing-runs", runs.RunRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/number-ranges", runs.NumberRangeRouter(ProtectedTraegerRoutes))
}

// NewBillingRunHTTP binds the billing runs to the right "Abrechnung", to
// Enrollment's holiday periods and the review of their registrations, their
// letters to the master data and branding settings of the Träger, and the
// mail of the letters to Delivery and the letterhead of the Träger.
func NewBillingRunHTTP(settings TraegerBrandingSettings, delivery deliveryRuntime.Capability, healthData enrollmentOwner.HolidayHealthCipher) *parentBillingHTTP.BillingRunResource {
	registrations := parentBillingBillingRegistrations{
		periods: enrollmentCompose.NewHolidayPeriods(), review: enrollmentCompose.NewHolidayRegistrationReview(healthData),
	}
	letterheads := parentBillingLetterheads{masterData: organizationTenancyCompose.NewTraegerMasterData(), settings: settings}
	return parentBillingHTTP.NewBillingRunResource(parentBillingCompose.NewBillingRuns(registrations),
		parentBillingCompose.NewBillingLetters(letterheads),
		parentBillingCompose.NewBillingLetterMail(parentBillingLetterPost{settings: settings, delivery: delivery}),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// parentBillingLetterPost binds the mail of the letters to Delivery: a mail
// of the Träger (ADR 0110) enqueued in the caller's Träger transaction with
// the record of the send, the run as its related entity and the PDF in its
// payload; and to the letterhead text of the branding settings of the
// Träger, whose first line names the sender (ADR 0112, ADR 0123).
type parentBillingLetterPost struct {
	settings TraegerBrandingSettings
	delivery deliveryRuntime.Capability
}

func (p parentBillingLetterPost) SenderText(ctx context.Context, organizationID int64) (string, error) {
	texts, err := p.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyBrandingLetterheadText})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyBrandingLetterheadText], nil
}

func (p parentBillingLetterPost) EnqueueLetterMail(ctx context.Context, mail parentbilling.BillingLetterMail) (int64, error) {
	payload, err := json.Marshal(mail)
	if err != nil {
		return 0, fmt.Errorf("letter mail payload: %w", err)
	}
	enqueued, err := p.delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: parentbilling.MailKindBillingLetter,
		Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient, Name: mail.GuardianName},
		Payload:   payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: parentbilling.MailRelatedBillingRun, ID: mail.RunID},
	})
	if err != nil {
		return 0, err
	}
	return enqueued.ID, nil
}

// EnqueueDocumentNotice puts the push of the Träger that a letter is in the
// inbox of the parents portal on Delivery's outbox, in the caller's Träger
// transaction (ADR 0126). Its audience is found when the worker sends it;
// the key of the letter makes a later send a duplicate.
func (p parentBillingLetterPost) EnqueueDocumentNotice(ctx context.Context, notice parentbilling.BillingDocumentNotice) error {
	payload, err := json.Marshal(notice)
	if err != nil {
		return fmt.Errorf("document notice payload: %w", err)
	}
	_, err = p.delivery.EnqueueDeferredPush(ctx, deliveryContract.DeferredPushIntent{
		OrganizationID: notice.OrganizationID, Template: parentbilling.PushKindBillingDocument,
		IdempotencyKey: fmt.Sprintf("parent-billing-document:%d", notice.DocumentID), Payload: payload,
		Related: deliveryContract.RelatedEntity{Type: parentbilling.PushRelatedBillingDocument, ID: notice.DocumentID},
	})
	return err
}

func (p parentBillingLetterPost) LetterMailStates(ctx context.Context, organizationID, runID int64) ([]parentbilling.BillingLetterMailState, error) {
	statuses, err := p.delivery.TraegerEmailStatuses(ctx, organizationID,
		deliveryContract.RelatedEntity{Type: parentbilling.MailRelatedBillingRun, ID: runID})
	if err != nil {
		return nil, err
	}
	states := make([]parentbilling.BillingLetterMailState, len(statuses))
	for i, status := range statuses {
		states[i] = parentbilling.BillingLetterMailState{
			DeliveryID: status.ID, Status: parentBillingMailStatus(status), Attempts: status.Attempts, SentAt: status.SentAt,
		}
	}
	return states, nil
}

// parentBillingMailStatus names Delivery's state of a mail in the
// Elternabrechnung's words: waiting, accepted by the mail server, given up,
// or unknown when the worker stopped during the provider call and the mail
// may have gone out.
func parentBillingMailStatus(status deliveryContract.Status) string {
	switch {
	case status.State == deliveryContract.StateSent:
		return parentbilling.LetterMailSent
	case status.State == deliveryContract.StateDeadLetter && status.LastError != nil &&
		*status.LastError == deliveryContract.DispatchOutcomeUnknown:
		return parentbilling.LetterMailUnknown
	case status.State == deliveryContract.StateDeadLetter, status.State == deliveryContract.StateCancelled:
		return parentbilling.LetterMailFailed
	default:
		return parentbilling.LetterMailQueued
	}
}

// TraegerBrandingSettings is the Träger level of the settings system as the
// letters read the branding of a Träger (ADR 0112).
type TraegerBrandingSettings interface {
	TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error)
}

// parentBillingLetterheads binds the letterhead of the letters to the
// master data of Organisation & Tenancy, which logs the read of the
// contact person and the bank accounts, and to the branding settings of
// the Träger (ADR 0112, ADR 0121).
type parentBillingLetterheads struct {
	masterData organizationtenancy.TraegerMasterData
	settings   TraegerBrandingSettings
}

func (b parentBillingLetterheads) Letterhead(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.Letterhead, error) {
	profile, err := b.masterData.Profile(ctx, reviewer.OrganizationID, reviewer.AccountID)
	if err != nil {
		return parentbilling.Letterhead{}, err
	}
	texts, err := b.settings.TraegerTexts(ctx, reviewer.OrganizationID,
		[]string{configModels.KeyBrandingPrimaryColor, configModels.KeyBrandingLetterheadText, configModels.KeyBrandingFooterText})
	if err != nil {
		return parentbilling.Letterhead{}, err
	}
	head := parentbilling.Letterhead{
		AccentColor: texts[configModels.KeyBrandingPrimaryColor], SenderText: texts[configModels.KeyBrandingLetterheadText],
		FooterText: texts[configModels.KeyBrandingFooterText], ContactName: profile.Contact.Name,
		ContactPhone: profile.Contact.Phone, ContactEmail: profile.Contact.Email,
	}
	for _, account := range profile.BankAccounts {
		if account.IsDefault {
			head.BankAccount = &parentbilling.LetterheadBankAccount{
				AccountHolder: account.AccountHolder, IBAN: account.IBAN, BIC: account.BIC, BankName: account.BankName,
			}
		}
	}
	if profile.Logo == nil {
		return head, nil
	}
	logo, err := b.masterData.LogoImage(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.Letterhead{}, err
	}
	head.Logo = logo.Data
	return head, nil
}

// NewParentBillingHTTP binds the Elternabrechnung's routes to the Träger
// rights and its holiday contribution to Enrollment's holiday
// registrations.
func NewParentBillingHTTP(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, healthData enrollmentOwner.HolidayHealthCipher, logger *slog.Logger) (*parentBillingHTTP.RuleResource, *parentBillingHTTP.HolidayResource) {
	registrations := parentBillingHolidayRegistrations{
		links:  newHolidayRegistrations(organizations, delivery, healthData, logger.With("service", "parent-billing-holiday-registrations")),
		review: enrollmentCompose.NewHolidayRegistrationReview(healthData),
	}
	rules := parentBillingHTTP.NewRuleResource(parentBillingCompose.NewContributionRules(),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
	holiday := parentBillingHTTP.NewHolidayResource(parentBillingCompose.NewHolidayContributions(registrations),
		common.RequiresPermission(identityaccess.TraegerRightEnrollmentManage))
	return rules, holiday
}

// parentBillingHolidayRegistrations binds the holiday contribution's port
// to Enrollment: the public link and its period, and the review of a
// registration, which checks the Träger and records every read and the
// confirmation in the registration's audit trail (ADR 0114).
type parentBillingHolidayRegistrations struct {
	links  enrollmentOwner.HolidayRegistrations
	review enrollmentOwner.HolidayRegistrationReview
}

func (b parentBillingHolidayRegistrations) ResolveHolidayLink(ctx context.Context, token string) (parentbilling.HolidayLink, error) {
	link, err := b.links.ResolveHolidayRegistrationLink(ctx, token)
	if err != nil {
		return parentbilling.HolidayLink{}, parentBillingHolidayError(err)
	}
	return parentbilling.HolidayLink{OrganizationID: link.OrganizationID, PeriodID: link.PeriodID}, nil
}

func (b parentBillingHolidayRegistrations) HolidayWeeks(ctx context.Context, link parentbilling.HolidayLink) ([]parentbilling.HolidayWeek, error) {
	offer, err := b.links.HolidayRegistrationOffer(ctx, enrollmentOwner.HolidayRegistrationLink{OrganizationID: link.OrganizationID, PeriodID: link.PeriodID})
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	weeks := make([]parentbilling.HolidayWeek, len(offer.Weeks))
	for i, week := range offer.Weeks {
		models := make([]parentbilling.HolidayTimeModel, len(week.TimeModels))
		for j, model := range week.TimeModels {
			models[j] = parentbilling.HolidayTimeModel{ID: model.ID, Name: model.Name, PriceCents: model.PriceCents}
		}
		weeks[i] = parentbilling.HolidayWeek{ID: week.ID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, TimeModels: models}
	}
	return weeks, nil
}

func (b parentBillingHolidayRegistrations) HolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.HolidayRegistration(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

func (b parentBillingHolidayRegistrations) ConfirmHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.ChangeHolidayRegistration(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, enrollmentOwner.HolidayAuditConfirm)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

// parentBillingBillingRegistrations binds the billing run's port to
// Enrollment: the period of the session's Träger and its confirmed
// registrations through the review, which records the read in the period's
// audit trail (ADR 0114).
type parentBillingBillingRegistrations struct {
	periods enrollmentOwner.HolidayPeriods
	review  enrollmentOwner.HolidayRegistrationReview
}

func (b parentBillingBillingRegistrations) HolidayBillingPeriods(ctx context.Context, reviewer parentbilling.HolidayReviewer) ([]parentbilling.HolidayBillingPeriod, error) {
	periods, err := b.periods.ListHolidayPeriods(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	billing := make([]parentbilling.HolidayBillingPeriod, len(periods))
	for i, period := range periods {
		billing[i] = parentBillingPeriod(period)
	}
	return billing, nil
}

func (b parentBillingBillingRegistrations) HolidayBillingPeriod(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) (parentbilling.HolidayBillingPeriod, error) {
	period, err := b.periods.HolidayPeriod(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return parentbilling.HolidayBillingPeriod{}, parentBillingHolidayError(err)
	}
	return parentBillingPeriod(period), nil
}

// parentBillingPeriod keeps the name of a period and the days of its first
// and last week.
func parentBillingPeriod(period enrollmentOwner.HolidayPeriod) parentbilling.HolidayBillingPeriod {
	billing := parentbilling.HolidayBillingPeriod{
		ID: period.ID, Name: period.Name, Published: period.Status == enrollmentOwner.HolidayPeriodPublished,
	}
	for _, week := range period.Weeks {
		if billing.StartsOn.IsZero() || week.StartsOn.Before(billing.StartsOn) {
			billing.StartsOn = week.StartsOn
		}
		if billing.EndsOn.IsZero() || week.EndsOn.After(billing.EndsOn) {
			billing.EndsOn = week.EndsOn
		}
	}
	return billing
}

func (b parentBillingBillingRegistrations) ConfirmedHolidayRegistrations(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) ([]parentbilling.HolidayBillingRegistration, error) {
	list, err := b.review.ListHolidayRegistrations(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID,
		enrollmentOwner.HolidayRegistrationFilter{Status: enrollmentOwner.HolidayRegistrationConfirmed})
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	registrations := make([]parentbilling.HolidayBillingRegistration, len(list.Registrations))
	for i, registration := range list.Registrations {
		children := make([]parentbilling.HolidayBillingChild, len(registration.Children))
		for j, child := range registration.Children {
			children[j] = parentbilling.HolidayBillingChild{ID: child.ID, FirstName: child.FirstName, LastName: child.LastName}
		}
		registrations[i] = parentbilling.HolidayBillingRegistration{
			ID: registration.ID, GuardianFirstName: registration.GuardianFirstName, GuardianLastName: registration.GuardianLastName,
			GuardianEmail: registration.GuardianEmail, Children: children,
		}
	}
	return registrations, nil
}

func parentBillingRegistration(detail enrollmentOwner.HolidayRegistrationDetail) parentbilling.HolidayRegistration {
	registration := parentbilling.HolidayRegistration{
		ID: detail.ID, Status: detail.Status, BenefitsDeclared: detail.BenefitsDeclared,
		Children: make([]parentbilling.HolidayChild, len(detail.Children)),
	}
	for i, child := range detail.Children {
		bookings := make([]parentbilling.HolidayBooking, len(child.Bookings))
		for j, booking := range child.Bookings {
			bookings[j] = parentbilling.HolidayBooking{
				WeekID: booking.WeekID, WeekStartsOn: booking.WeekStartsOn, WeekEndsOn: booking.WeekEndsOn,
				TimeModelID: booking.TimeModelID, TimeModelName: booking.TimeModelName, PriceCents: booking.PriceCents,
			}
		}
		registration.Children[i] = parentbilling.HolidayChild{ID: child.ID, Position: i + 1, Bookings: bookings}
	}
	return registration
}

// parentBillingHolidayError names Enrollment's refusals in the
// Elternabrechnung's vocabulary; anything else stays as it is.
func parentBillingHolidayError(err error) error {
	for _, known := range []struct{ from, to error }{
		{enrollmentOwner.ErrHolidayRegistrationNotFound, parentbilling.ErrHolidayLinkNotFound},
		{enrollmentOwner.ErrHolidayPeriodNotFound, parentbilling.ErrHolidayPeriodNotFound},
		{enrollmentOwner.ErrHolidayRegistrationEntryNotFound, parentbilling.ErrHolidayRegistrationNotFound},
		{enrollmentOwner.ErrHolidayRegistrationStateConflict, parentbilling.ErrHolidayRegistrationStateConflict},
	} {
		if errors.Is(err, known.from) {
			return fmt.Errorf("%w: %w", known.to, err)
		}
	}
	return err
}
