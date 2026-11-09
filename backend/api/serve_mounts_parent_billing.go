package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationTenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	parentBillingHTTP "github.com/moto-nrw/project-phoenix/modules/parentbilling/http"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mountParentBillingRoutes serves the Elternabrechnung (ADR 0088, #4664, ADR
// 0115): the contribution rules of a Träger at /traeger/contribution-rules
// behind the right "Abrechnung", the contribution and confirmation of a
// holiday registration at /traeger/holiday-contributions behind the right
// "Anmeldung verwalten", and the public contribution of the registration
// form at /traeger/holiday-quotes, the cost bearers of a Träger at
// /traeger/cost-bearers behind the right "Abrechnung" (#5627), and the
// billing runs and number ranges at /traeger/billing-runs and
// /traeger/number-ranges behind the right "Abrechnung" (#4668, ADR 0119),
// with the confirmation letters of a released run in the layout of the
// Träger (#4669, ADR 0121), its overview for the Kämmerei (#4672, ADR 0122)
// and the list for the reimbursement by a cost bearer (#5627), and the mail
// of the letters as mails of the Träger on Delivery's outbox and their
// print in one file (#4670, ADR 0123), and the invoices and amounts per
// family at /traeger/billing-families behind the right "Abrechnung" (#4691),
// and the children of the families at /traeger/family-children (#5918),
// and the payment terms, the family deviations and the due dates of the runs
// at /traeger/billing-terms, /traeger/family-billing and
// /traeger/billing-run-due-dates behind the same right (#4689),
// and the kind of document of each service at
// /traeger/billing-document-kinds behind the right "Abrechnung" (#4688),
// and the Jahresbescheide at /traeger/care-notice-runs behind the same
// right (#6219),
// and the SEPA mandates and the creditor at /traeger/sepa-mandates and
// /traeger/sepa-creditor behind the same right (#4695), and the SEPA direct
// debit file of the issued runs at /traeger/sepa-debits (#4697).
func mountParentBillingRoutes(root chi.Router, organizations HolidayOrganizations, settings TraegerBrandingSettings, schoolSettings owners.MealAttendanceSchoolSettings, marks owners.MealAttendanceMarks, delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, healthData enrollmentOwner.HolidayHealthCipher, bankData parentbilling.BankDataCipher, logger *slog.Logger) {
	rules, holiday := NewParentBillingHTTP(organizations, settings, delivery, healthData, logger)
	root.Mount("/traeger/contribution-rules", rules.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/cost-bearers", rules.CostBearerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-contributions", holiday.TraegerRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/holiday-quotes", holiday.QuoteRouter())
	runs, documentKinds := NewBillingRunHTTP(settings, delivery, outcomes, healthData)
	root.Mount("/traeger/billing-runs", runs.RunRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/number-ranges", runs.NumberRangeRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/billing-document-kinds", documentKinds.Router(ProtectedTraegerRoutes))
	families := parentBillingHTTP.NewBillingFamilyResource(parentBillingCompose.NewBillingFamilies(),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
	root.Mount("/traeger/billing-families", families.Router(ProtectedTraegerRoutes))
	care, reductions, meals, familyChildren := NewCareBillingHTTP(organizations, settings, schoolSettings, marks, delivery, outcomes)
	root.Mount("/traeger/care-tariffs", care.TariffRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/care-billing-runs", care.RunRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/care-rules", reductions.RuleRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/care-evidences", reductions.EvidenceRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/care-meal-tariffs", meals.Router(ProtectedTraegerRoutes))
	root.Mount("/traeger/care-cost-shares", reductions.CostShareRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/family-children", familyChildren.Router(ProtectedTraegerRoutes))
	root.Mount("/traeger/care-notice-runs", NewCareNoticeHTTP(organizations, settings, schoolSettings, marks).Router(ProtectedTraegerRoutes))
	terms := NewBillingTermsHTTP()
	root.Mount("/traeger/billing-terms", terms.TermsRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/family-billing", terms.FamilyRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/billing-run-due-dates", terms.DueDateRouter(ProtectedTraegerRoutes))
	mandates := NewSEPAMandateHTTP(bankData)
	root.Mount("/traeger/sepa-creditor", mandates.CreditorRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/sepa-mandates", mandates.MandateRouter(ProtectedTraegerRoutes))
	root.Mount("/traeger/sepa-debits", NewSEPADebitHTTP(settings, bankData).Router(ProtectedTraegerRoutes))
}

// NewSEPADebitHTTP binds the SEPA direct debit file of the issued runs
// (#4697, ADR 0181) to the right "Abrechnung", to the letterhead with the
// default bank account of the Träger, whose read the master data of
// Organisation & Tenancy logs, and to the field cipher that sealed the
// account of each mandate.
func NewSEPADebitHTTP(settings TraegerBrandingSettings, cipher parentbilling.BankDataCipher) *parentBillingHTTP.SEPADebitResource {
	return parentBillingHTTP.NewSEPADebitResource(parentBillingCompose.NewSEPADebits(newParentBillingLetterheads(settings), cipher),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// NewSEPAMandateHTTP binds the SEPA mandates and the creditor of the Träger
// (#4695, ADR 0171) to the right "Abrechnung", to the parties of the
// Betreuungsvertrag, which name the family of a contract, and to the field
// independent bank-data cipher of the root.
func NewSEPAMandateHTTP(cipher parentbilling.BankDataCipher) *parentBillingHTTP.SEPAMandateResource {
	contracts := parentBillingCareContracts{contracts: careContractCompose.NewContractVersions()}
	return parentBillingHTTP.NewSEPAMandateResource(parentBillingCompose.NewSEPAMandates(contracts, cipher),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// NewBillingTermsHTTP binds the payment terms, the bundling, the family
// deviations and the due dates of the runs (#4689, ADR 0167) to the right
// "Abrechnung" and to the parties of the Betreuungsvertrag, which name the
// family of a contract, in the caller's Träger transaction.
func NewBillingTermsHTTP() *parentBillingHTTP.BillingTermsResource {
	contracts := parentBillingCareContracts{contracts: careContractCompose.NewContractVersions()}
	return parentBillingHTTP.NewBillingTermsResource(parentBillingCompose.NewBillingTerms(contracts),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// NewCareBillingHTTP binds the monthly contribution of the care contract
// (#4684), its rule chain (#4685), the children of the families (#5918) and
// its Mensa prices (#4686) to the right "Abrechnung", to the versions in force
// and the parties of the Betreuungsvertrag in the caller's Träger transaction,
// to the schools of the Träger, to its month rule and to the released Mensa
// months of Meal Plan; its letters, their mail and Storno to the letterhead of
// the Träger and to Delivery like the holiday letters (ADR 0165).
func NewCareBillingHTTP(organizations HolidayOrganizations, settings TraegerBrandingSettings, schoolSettings owners.MealAttendanceSchoolSettings,
	marks owners.MealAttendanceMarks, delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes,
) (*parentBillingHTTP.CareBillingResource, *parentBillingHTTP.CareReductionResource, *parentBillingHTTP.CareMealTariffResource, *parentBillingHTTP.FamilyChildrenResource) {
	contracts := parentBillingCareContracts{contracts: careContractCompose.NewContractVersions()}
	billing, reductions := parentBillingCompose.NewCareBilling(contracts,
		parentBillingCareSchools{organizations: organizations}, parentBillingSettings{settings: settings}, newParentBillingCareMeals(schoolSettings, marks),
		newParentBillingLetterheads(settings))
	post := parentBillingLetterPost{settings: settings, delivery: delivery, outcomes: outcomes}
	letters := parentBillingCompose.NewCareLetters(newParentBillingLetterheads(settings), contracts, post, post)
	requireBilling := common.RequiresPermission(identityaccess.TraegerRightBillingManage)
	return parentBillingHTTP.NewCareBillingResource(billing, letters, requireBilling), parentBillingHTTP.NewCareReductionResource(reductions, requireBilling),
		parentBillingHTTP.NewCareMealTariffResource(billing, requireBilling), parentBillingHTTP.NewFamilyChildrenResource(reductions, requireBilling)
}

// NewCareNoticeHTTP binds the Jahresbescheide of the monthly care (#6219,
// ADR 0182) to the right "Abrechnung" and to the same ports as the monthly
// run, plus the letterhead of the Träger for page 1 of the Bescheid.
func NewCareNoticeHTTP(organizations HolidayOrganizations, settings TraegerBrandingSettings, schoolSettings owners.MealAttendanceSchoolSettings,
	marks owners.MealAttendanceMarks,
) *parentBillingHTTP.CareNoticeResource {
	contracts := parentBillingCareContracts{contracts: careContractCompose.NewContractVersions()}
	notices := parentBillingCompose.NewCareFeeNotices(contracts, parentBillingCareSchools{organizations: organizations},
		parentBillingSettings{settings: settings}, newParentBillingCareMeals(schoolSettings, marks), newParentBillingLetterheads(settings))
	return parentBillingHTTP.NewCareNoticeResource(notices, common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// parentBillingCareSchools lists the schools of the Träger at Organisation
// & Tenancy that are not deleted, by name, for the price list (#4684).
type parentBillingCareSchools struct {
	organizations HolidayOrganizations
}

func (s parentBillingCareSchools) TraegerSchools(ctx context.Context, organizationID int64) ([]parentbilling.CareSchool, error) {
	schools, err := s.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]parentbilling.CareSchool, 0, len(schools))
	for _, school := range schools {
		if !school.IsDeleted() {
			result = append(result, parentbilling.CareSchool{ID: school.ID, Name: school.Name})
		}
	}
	slices.SortFunc(result, func(a, b parentbilling.CareSchool) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

// parentBillingCareContracts translates the Betreuungsvertrag's versions in
// force and invoice recipients into the Elternabrechnung's port.
type parentBillingCareContracts struct {
	contracts carecontract.ContractBilling
}

func (c parentBillingCareContracts) VersionsInForce(ctx context.Context, organizationID int64, from, to calendar.Date) ([]parentbilling.CareContractVersion, error) {
	periods, err := c.contracts.VersionsInForce(ctx, organizationID, from, to)
	if err != nil {
		return nil, err
	}
	versions := make([]parentbilling.CareContractVersion, len(periods))
	for i, period := range periods {
		services := make([]parentbilling.CareContractService, len(period.Version.Services))
		for j, service := range period.Version.Services {
			services[j] = parentbilling.CareContractService{Weekday: service.Weekday, CareOfferingID: service.CareOfferingID,
				ModuleName: service.ModuleName, EndsAt: service.EndsAt, LastDay: service.LastDay}
		}
		versions[i] = parentbilling.CareContractVersion{
			ContractID: period.ContractID, SchoolID: period.SchoolID, SchoolName: period.Version.SchoolName,
			Number: period.Version.Number, ChildFirstName: period.Version.ChildFirstName, ChildLastName: period.Version.ChildLastName,
			StudentID: period.StudentID, ChildBirthDate: period.Version.ChildBirthDate,
			From: period.From, Until: period.Until, Services: services,
		}
	}
	return versions, nil
}

// Recipients translates the party each invoice goes to.
func (c parentBillingCareContracts) Recipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]parentbilling.CareRecipient, error) {
	parties, err := c.contracts.BillingRecipients(ctx, organizationID, contractIDs)
	if err != nil {
		return nil, err
	}
	recipients := make(map[int64]parentbilling.CareRecipient, len(parties))
	for contractID, party := range parties {
		recipients[contractID] = parentbilling.CareRecipient{FirstName: party.FirstName, LastName: party.LastName, Email: party.Email}
	}
	return recipients, nil
}

// NewBillingRunHTTP binds the billing runs to the right "Abrechnung", to
// Enrollment's holiday periods and the review of their registrations, their
// letters to the master data and branding settings of the Träger, and the
// mail of the letters to Delivery and the letterhead of the Träger; and the
// kind of document per service (#4688) to the document templates of the
// Träger.
func NewBillingRunHTTP(settings TraegerBrandingSettings, delivery deliveryRuntime.Capability, outcomes deliveryRuntime.EmailOutcomes, healthData enrollmentOwner.HolidayHealthCipher) (*parentBillingHTTP.BillingRunResource, *parentBillingHTTP.DocumentKindResource) {
	review, _ := enrollmentCompose.NewHolidayRegistrationReview(healthData, nil)
	registrations := parentBillingBillingRegistrations{periods: enrollmentCompose.NewHolidayPeriods(nil), review: review}
	requireBilling := common.RequiresPermission(identityaccess.TraegerRightBillingManage)
	runs, kinds := parentBillingCompose.NewBillingRuns(registrations, parentBillingSettings{settings: settings}, newParentBillingLetterheads(settings))
	return parentBillingHTTP.NewBillingRunResource(runs,
			parentBillingCompose.NewBillingLetters(newParentBillingLetterheads(settings)),
			parentBillingCompose.NewBillingLetterMail(parentBillingLetterPost{settings: settings, delivery: delivery, outcomes: outcomes}),
			requireBilling),
		parentBillingHTTP.NewDocumentKindResource(kinds, requireBilling)
}

// parentBillingLetterPost binds the mail of the letters to Delivery: a mail
// of the Träger (ADR 0110) enqueued in the caller's Träger transaction with
// the record of the send, the run as its related entity and the PDF in its
// payload; and to the letterhead text of the branding settings of the
// Träger, whose first line names the sender (ADR 0112, ADR 0123).
type parentBillingLetterPost struct {
	settings TraegerBrandingSettings
	delivery deliveryRuntime.Capability
	outcomes deliveryRuntime.EmailOutcomes
}

// LetterMailBlocks reads the provider's block as it is now.
func (p parentBillingLetterPost) LetterMailBlocks(ctx context.Context, addresses []string) map[string]string {
	blocks := make(map[string]string, len(addresses))
	for address, block := range p.outcomes.ProviderEmailBlocks(ctx, addresses) {
		blocks[address] = string(block)
	}
	return blocks
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
			DeliveryID: status.ID,
			Status:     parentBillingMailStatus(status),
			Attempts:   status.Attempts,
			SentAt:     status.SentAt,
			Outcome:    string(status.Outcome),
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
// letters read the branding of a Träger (ADR 0112) and the Elternabrechnung
// its switches (#5621).
type TraegerBrandingSettings interface {
	TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error)
	TraegerBools(ctx context.Context, organizationID int64, keys []string) (map[string]bool, error)
}

// parentBillingSettings binds the Elternabrechnung's settings port to the
// Träger level of the settings system: whether the Träger enters the debtor
// number of the family when it confirms a holiday registration (#5621), its
// payment text, which the billing run checks before the release (#5622), and
// its choices in the chain of the holiday contribution (#5664).
type parentBillingSettings struct {
	settings TraegerBrandingSettings
}

func (s parentBillingSettings) AsksDebtorNumber(ctx context.Context, organizationID int64) (bool, error) {
	values, err := s.settings.TraegerBools(ctx, organizationID, []string{configModels.KeyParentBillingDebtorNumberOnConfirm})
	if err != nil {
		return false, err
	}
	return values[configModels.KeyParentBillingDebtorNumberOnConfirm], nil
}

func (s parentBillingSettings) PaymentText(ctx context.Context, organizationID int64) (string, error) {
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyParentBillingPaymentText})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyParentBillingPaymentText], nil
}

func (s parentBillingSettings) HolidayChain(ctx context.Context, organizationID int64) (parentbilling.HolidayChainSettings, error) {
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{
		configModels.KeyParentBillingSiblingRank, configModels.KeyParentBillingCostTakeoverOrder,
	})
	if err != nil {
		return parentbilling.HolidayChainSettings{}, err
	}
	return parentbilling.HolidayChainSettings{
		SiblingRank:       texts[configModels.KeyParentBillingSiblingRank],
		CostTakeoverOrder: texts[configModels.KeyParentBillingCostTakeoverOrder],
	}, nil
}

// CareMonthRule is the Träger's value of parent_billing.care_month_change
// or its default (#4684).
func (s parentBillingSettings) CareMonthRule(ctx context.Context, organizationID int64) (string, error) {
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyParentBillingCareMonthChange})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyParentBillingCareMonthChange], nil
}

// CareAmendable reads which changes of a single invoice the review of a
// monthly run allows (#6197): the Träger's switches or their defaults.
func (s parentBillingSettings) CareAmendable(ctx context.Context, organizationID int64) (parentbilling.CareAmendable, error) {
	values, err := s.settings.TraegerBools(ctx, organizationID, []string{
		configModels.KeyParentBillingCareReviewAmount, configModels.KeyParentBillingCareReviewRecipient,
		configModels.KeyParentBillingCareReviewAddress, configModels.KeyParentBillingCareReviewPositions,
	})
	if err != nil {
		return parentbilling.CareAmendable{}, err
	}
	return parentbilling.CareAmendable{
		Amount:    values[configModels.KeyParentBillingCareReviewAmount],
		Recipient: values[configModels.KeyParentBillingCareReviewRecipient],
		Address:   values[configModels.KeyParentBillingCareReviewAddress],
		Positions: values[configModels.KeyParentBillingCareReviewPositions],
	}, nil
}

// parentBillingLetterheads binds the letterhead of the letters to the
// master data of Organisation & Tenancy, which logs the read of the
// contact person and the bank accounts, to the branding settings and the
// payment text of the Träger (ADR 0112, ADR 0121, #5622), and the template
// of the confirmation letter to the document templates of the Träger
// (#4677, ADR 0152).
type parentBillingLetterheads struct {
	masterData organizationtenancy.TraegerMasterData
	templates  organizationtenancy.TraegerDocumentTemplates
	settings   TraegerBrandingSettings
}

// newParentBillingLetterheads builds the letterhead of the holiday and the
// monthly letters.
func newParentBillingLetterheads(settings TraegerBrandingSettings) parentBillingLetterheads {
	return parentBillingLetterheads{masterData: organizationTenancyCompose.NewTraegerMasterData(),
		templates: organizationTenancyCompose.NewTraegerDocumentTemplates(), settings: settings}
}

func (b parentBillingLetterheads) Letterhead(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.Letterhead, error) {
	profile, err := b.masterData.Profile(ctx, reviewer.OrganizationID, reviewer.AccountID)
	if err != nil {
		return parentbilling.Letterhead{}, err
	}
	texts, err := b.settings.TraegerTexts(ctx, reviewer.OrganizationID, []string{
		configModels.KeyBrandingPrimaryColor, configModels.KeyBrandingLetterheadText, configModels.KeyBrandingFooterText,
		configModels.KeyParentBillingPaymentText,
	})
	if err != nil {
		return parentbilling.Letterhead{}, err
	}
	head := parentbilling.Letterhead{
		AccentColor: texts[configModels.KeyBrandingPrimaryColor], SenderText: texts[configModels.KeyBrandingLetterheadText],
		FooterText: texts[configModels.KeyBrandingFooterText], ContactName: profile.Contact.Name,
		ContactPhone: profile.Contact.Phone, ContactEmail: profile.Contact.Email,
		PaymentText: texts[configModels.KeyParentBillingPaymentText],
	}
	template, err := b.templates.Current(ctx, reviewer.OrganizationID, templates.KindConfirmation)
	if err != nil {
		return parentbilling.Letterhead{}, err
	}
	head.Template = parentbilling.LetterTemplate{
		Version: template.Version, Subject: template.Subject, Salutation: template.Salutation,
		Body: template.Body, Closing: template.Closing,
		GroupByChild: template.Invoice.GroupByChild, HideTimeModels: template.Invoice.HideTimeModels,
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

// parentBillingTemplateKinds maps the kinds of document of the billing to
// the kinds of template of Document Rendering (#4688, ADR 0152).
var parentBillingTemplateKinds = map[string]templates.Kind{
	parentbilling.DocumentKindConfirmation: templates.KindConfirmation,
	parentbilling.DocumentKindInvoice:      templates.KindInvoice,
	parentbilling.DocumentKindFeeNotice:    templates.KindFeeNotice,
}

// DocumentTemplate reads the Träger's current template of a kind of
// document, or its standard one, in the Träger transaction; the template
// names no person (#4688).
func (b parentBillingLetterheads) DocumentTemplate(ctx context.Context, organizationID int64, documentKind string) (parentbilling.LetterTemplate, error) {
	kind, ok := parentBillingTemplateKinds[documentKind]
	if !ok {
		return parentbilling.LetterTemplate{}, parentbilling.ErrDocumentKindInvalid
	}
	template, err := b.templates.Current(ctx, organizationID, kind)
	if err != nil {
		return parentbilling.LetterTemplate{}, err
	}
	return parentbilling.LetterTemplate{
		Version: template.Version, Subject: template.Subject, Salutation: template.Salutation,
		Body: template.Body, Closing: template.Closing,
		GroupByChild: template.Invoice.GroupByChild, HideTimeModels: template.Invoice.HideTimeModels,
		LegalRemedy: template.LegalRemedy,
	}, nil
}

// NewParentBillingHTTP binds the Elternabrechnung's routes to the Träger
// rights, its holiday contribution to Enrollment's holiday registrations and
// to the Träger's switch for the debtor number (#5621).
func NewParentBillingHTTP(organizations HolidayOrganizations, settings TraegerBrandingSettings, delivery deliveryRuntime.Capability, healthData enrollmentOwner.HolidayHealthCipher, logger *slog.Logger) (*parentBillingHTTP.RuleResource, *parentBillingHTTP.HolidayResource) {
	review, places := enrollmentCompose.NewHolidayRegistrationReview(healthData, holidayTraegerMail{delivery: delivery})
	registrations := parentBillingHolidayRegistrations{
		links:  newHolidayRegistrations(organizations, delivery, healthData, logger.With("service", "parent-billing-holiday-registrations")),
		review: review, places: places,
	}
	rules := parentBillingHTTP.NewRuleResource(parentBillingCompose.NewContributionRules(),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
	holiday := parentBillingHTTP.NewHolidayResource(parentBillingCompose.NewHolidayContributions(registrations, parentBillingSettings{settings: settings}),
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
	places enrollmentOwner.HolidayPlaces
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

func (b parentBillingHolidayRegistrations) ConfirmHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, debtorNumber string) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.ConfirmHolidayRegistration(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

func (b parentBillingHolidayRegistrations) SetHolidayDebtorNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, debtorNumber string) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.SetHolidayDebtorNumber(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

// CancelHolidayRegistration cancels the registration through the review,
// which checks the Träger and the deadline and records the cancellation
// (#5662).
func (b parentBillingHolidayRegistrations) CancelHolidayRegistration(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayRegistration, error) {
	detail, err := b.review.ChangeHolidayRegistration(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, enrollmentOwner.HolidayAuditCancel)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingHolidayError(err)
	}
	return parentBillingRegistration(detail), nil
}

// RebookHolidayChild rebooks the child through the review, which gives the
// places it frees to the waiting lists (#4699).
func (b parentBillingHolidayRegistrations) RebookHolidayChild(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID, childID int64, bookings []parentbilling.HolidayBookingChoice) (parentbilling.HolidayRegistration, error) {
	choices := make([]enrollmentOwner.HolidayBookingInput, len(bookings))
	for i, booking := range bookings {
		choices[i] = enrollmentOwner.HolidayBookingInput{WeekID: booking.WeekID, TimeModelID: booking.TimeModelID}
	}
	detail, err := b.review.RebookHolidayChild(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, childID, choices)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingPlacesError(err)
	}
	registration := parentBillingRegistration(detail)
	registration.View = detail
	return registration, nil
}

// SetHolidayWeekCapacity sets the places of a week through Enrollment
// (#4699) and names the registrations that moved up.
func (b parentBillingHolidayRegistrations) SetHolidayWeekCapacity(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, weekID int64, capacity *int) ([]int64, error) {
	change, err := b.places.SetHolidayWeekCapacity(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, weekID, capacity)
	if err != nil {
		return nil, parentBillingPlacesError(err)
	}
	return promotedRegistrations(change.Promotions), nil
}

// parentBillingPlacesError names a refused rebooking or capacity in the
// Elternabrechnung's vocabulary (#4699).
func parentBillingPlacesError(err error) error {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		return fmt.Errorf("%w: %w", parentbilling.ErrInvalidHolidayPlaces, err)
	case errors.Is(err, enrollmentOwner.ErrHolidayWeekNotFound):
		return fmt.Errorf("%w: %w", parentbilling.ErrHolidayWeekNotFound, err)
	}
	return parentBillingHolidayError(err)
}

func promotedRegistrations(promotions []enrollmentOwner.HolidayPromotion) []int64 {
	ids := []int64{}
	for _, promotion := range promotions {
		if !slices.Contains(ids, promotion.RegistrationID) {
			ids = append(ids, promotion.RegistrationID)
		}
	}
	return ids
}

func (b parentBillingHolidayRegistrations) HoldHolidayPeriod(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) error {
	return parentBillingHolidayError(b.review.HoldHolidayPeriod(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID))
}

func (b parentBillingHolidayRegistrations) RecordHolidaySiblings(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, action string, siblingIDs []int64) error {
	return parentBillingHolidayError(b.review.RecordHolidaySiblings(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, action, siblingIDs))
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
	confirmed, err := b.review.ListHolidayRegistrationRecipients(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID,
		enrollmentOwner.HolidayRegistrationFilter{Status: enrollmentOwner.HolidayRegistrationConfirmed})
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	registrations := make([]parentbilling.HolidayBillingRegistration, len(confirmed))
	for i, registration := range confirmed {
		children := make([]parentbilling.HolidayBillingChild, len(registration.Children))
		for j, child := range registration.Children {
			children[j] = parentbilling.HolidayBillingChild{ID: child.ID, FirstName: child.FirstName, LastName: child.LastName}
		}
		registrations[i] = parentbilling.HolidayBillingRegistration{
			ID: registration.ID, GuardianFirstName: registration.GuardianFirstName, GuardianLastName: registration.GuardianLastName,
			GuardianEmail: registration.GuardianEmail, DebtorNumber: registration.DebtorNumber, Children: children,
		}
		if address := registration.GuardianAddress; address != nil {
			registrations[i].GuardianAddress = &parentbilling.PostalAddress{Street: address.Street, PostalCode: address.PostalCode, City: address.City}
		}
	}
	return registrations, nil
}

// HolidayProofRegistrations reads every registration of the period through
// the review, with no status filter, so a registration cancelled after its
// invoice still names its proof (#5627); the review checks the Träger and
// records the read like the list.
func (b parentBillingBillingRegistrations) HolidayProofRegistrations(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID int64) (map[int64]bool, error) {
	registrations, err := b.review.ListHolidayRegistrationRecipients(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID,
		enrollmentOwner.HolidayRegistrationFilter{})
	if err != nil {
		return nil, parentBillingHolidayError(err)
	}
	proofs := make(map[int64]bool, len(registrations))
	for _, registration := range registrations {
		proofs[registration.ID] = registration.HasBenefitsProof
	}
	return proofs, nil
}

func parentBillingRegistration(detail enrollmentOwner.HolidayRegistrationDetail) parentbilling.HolidayRegistration {
	registration := parentbilling.HolidayRegistration{
		ID: detail.ID, Status: detail.Status, SubmittedAt: detail.SubmittedAt, BenefitsDeclared: detail.BenefitsDeclared,
		DebtorNumber: detail.DebtorNumber, Children: make([]parentbilling.HolidayChild, len(detail.Children)),
		MovedUp: promotedRegistrations(detail.Promotions),
	}
	for i, child := range detail.Children {
		bookings := make([]parentbilling.HolidayBooking, len(child.Bookings))
		for j, booking := range child.Bookings {
			bookings[j] = parentbilling.HolidayBooking{
				WeekID: booking.WeekID, WeekStartsOn: booking.WeekStartsOn, WeekEndsOn: booking.WeekEndsOn,
				TimeModelID: booking.TimeModelID, TimeModelName: booking.TimeModelName, PriceCents: booking.PriceCents,
				Waitlisted: booking.Placement == enrollmentOwner.HolidayPlacementWaitlisted,
			}
		}
		registration.Children[i] = parentbilling.HolidayChild{
			ID: child.ID, Position: i + 1, FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate, Bookings: bookings,
		}
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
		{enrollmentOwner.ErrHolidayRegistrationFrozen, parentbilling.ErrHolidayRegistrationFrozen},
	} {
		if errors.Is(err, known.from) {
			return fmt.Errorf("%w: %w", known.to, err)
		}
	}
	return err
}
