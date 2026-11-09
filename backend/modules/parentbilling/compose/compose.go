// Package compose builds the Elternabrechnung (parent-billing, ADR 0088).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/rendering"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewContributionRules builds the rules and cost bearers (#5627) of a Träger in its Träger transaction (ADR 0105).
func NewContributionRules() parentbilling.ContributionRules {
	return application.NewRules(postgres.NewStore())
}

// NewHolidayContributions builds the holiday contribution (#4664, ADR 0115) over the registrations the root binds.
func NewHolidayContributions(registrations parentbilling.HolidayRegistrations, settings parentbilling.HolidayBillingSettings) parentbilling.HolidayContributions {
	return application.NewHolidayContributions(postgres.NewStore(), registrations, settings)
}

// NewBillingRuns builds the billing runs (#4668, ADR 0119, ADR 0122) and the kind of document per service (#4688) over the registrations and document templates the root binds.
func NewBillingRuns(registrations parentbilling.HolidayBillingRegistrations, settings parentbilling.HolidayBillingSettings, templates parentbilling.BillingDocumentTemplates) (parentbilling.BillingRuns, parentbilling.BillingDocumentKinds) {
	return application.NewBillingRuns(postgres.NewStore(), registrations, settings, rendering.NewRenderer(), templates, calendar.TodayDate), application.NewDocumentKinds(postgres.NewStore(), templates)
}

// NewBillingLetterMail builds the mail and print of the letters (#4670, ADR 0123) over the post the root binds.
func NewBillingLetterMail(post parentbilling.BillingLetterPost) parentbilling.BillingLetterMailService {
	return application.NewBillingLetterMail(postgres.NewStore(), post, rendering.NewRenderer())
}

// NewLetterMailRenderer renders the mail of a letter for the Delivery worker.
func NewLetterMailRenderer() parentbilling.LetterMailRenderer { return application.RenderLetterMail }

// NewBillingLetters builds the confirmation letters of the released runs
// (#4669, ADR 0121) over the letterhead of the Träger, which the root
// binds, and the letter layout of Document Rendering.
func NewBillingLetters(letterheads parentbilling.BillingLetterheads) parentbilling.BillingLetterService {
	return application.NewBillingLetters(postgres.NewStore(), letterheads, rendering.NewRenderer(), calendar.TodayDate)
}

// NewParentDocuments builds the inbox of the parents portal (#4671, ADR 0126) in the Träger transaction it opens.
func NewParentDocuments() *application.ParentDocuments {
	return application.NewParentDocuments(postgres.NewStore())
}

// NewBillingFamilies builds the invoices and amounts per family (#4691) in the Träger transaction.
func NewBillingFamilies() parentbilling.BillingFamilies {
	return application.NewBillingFamilies(postgres.NewStore())
}

// MonthlyBilling is the monthly run with its Mensa prices (#4684, #4686).
type MonthlyBilling interface {
	parentbilling.CareBilling
	parentbilling.CareMealBilling
}

// NewCareBilling builds the monthly contribution of the care contract (#4684), its rule chain (#4685) and its Mensa (#4686) over the contracts, schools, settings and released Mensa months the root binds, and the document templates (#4688).
func NewCareBilling(contracts parentbilling.CareContracts, schools parentbilling.CareSchools, settings parentbilling.CareBillingSettings, meals parentbilling.CareMeals, templates parentbilling.BillingDocumentTemplates) (MonthlyBilling, parentbilling.CareReductions) {
	store := postgres.NewStore()
	return application.NewCareBilling(store, contracts, schools, settings, rendering.NewRenderer(), store, meals, templates, calendar.TodayDate), application.NewCareReductions(postgres.NewStore(), contracts, schools, calendar.TodayDate)
}

// NewCareLetters builds the letters, mail, print and Storno of the monthly runs (#4684, ADR 0165) over the ports the root binds.
func NewCareLetters(letterheads parentbilling.BillingLetterheads, contracts parentbilling.CareContracts, post parentbilling.BillingLetterPost, carePost parentbilling.CareLetterPost) parentbilling.CareLetterService {
	return application.NewCareLetters(postgres.NewStore(), application.CareLetterPorts{
		Letterheads: letterheads, Recipients: contracts, Post: post, CarePost: carePost,
	}, rendering.NewRenderer(), rendering.NewRenderer(), calendar.TodayDate)
}

// NewBillingTerms builds the payment terms, the bundling, the family deviations and the due dates of the runs (#4689) over the contracts the root binds.
func NewBillingTerms(contracts parentbilling.CareContracts) parentbilling.BillingTerms {
	return application.NewBillingTerms(postgres.NewStore(), contracts)
}

// NewCareFeeNotices builds the Jahresbescheide of the monthly care (#6219, ADR 0182) over the monthly computation and the letterhead of the Träger, which carries the template of the fee notice; the root binds the ports.
func NewCareFeeNotices(contracts parentbilling.CareContracts, schools parentbilling.CareSchools, settings parentbilling.CareBillingSettings, meals parentbilling.CareMeals, letterheads parentbilling.BillingLetterheads) parentbilling.CareFeeNotices {
	store := postgres.NewStore()
	billing := application.NewCareBilling(store, contracts, schools, settings, rendering.NewRenderer(), store, meals, letterheads, calendar.TodayDate)
	return application.NewCareFeeNotices(billing, store, letterheads)
}

// NewSEPAMandates builds the SEPA mandates and the creditor of the Träger (#4695, ADR 0171) over the contracts and the field cipher the root binds.
func NewSEPAMandates(contracts parentbilling.CareContracts, cipher parentbilling.BankDataCipher) parentbilling.SEPAMandates {
	return application.NewSEPAMandates(postgres.NewStore(), contracts, cipher, calendar.TodayDate)
}

// NewSEPAExportFields opens the sealed IBAN and account holder for the return export of a Träger (#3838); only the administrative export CLI binds it, never HTTP.
func NewSEPAExportFields(cipher parentbilling.BankDataCipher) parentbilling.SEPAExportFields {
	return application.NewSEPAExportFields(cipher)
}

// NewBankDataRekey builds maintenance-only re-encryption; never bind it to HTTP.
func NewBankDataRekey(cipher parentbilling.BankDataRewrapper) parentbilling.BankDataRekey {
	return application.NewBankDataRekey(postgres.NewBankDataRekeyStore(), cipher)
}

// NewSEPAReturn builds the bank data of the Rückgabe of a Träger (#6314)
// over its creditor account and the field cipher the root binds. Internal
// procedure only; never bind it to HTTP.
func NewSEPAReturn(creditors parentbilling.SEPACreditorAccounts, cipher parentbilling.BankDataCipher, fingerprint func([]byte) string) parentbilling.SEPAReturn {
	return application.NewSEPAReturn(postgres.NewStore(), creditors, cipher, fingerprint)
}

// NewSEPADebitDownloadRetention builds the hourly expiry of the unsubmitted PAIN.008 downloads (ADR 0187).
func NewSEPADebitDownloadRetention() parentbilling.SEPADebitDownloadRetention {
	return postgres.NewStore()
}

// NewSEPADebits builds the SEPA direct debit file of the issued runs (#4697, ADR 0181) over the letterhead with the bank account of the Träger and the field cipher the root binds.
func NewSEPADebits(letterheads parentbilling.BillingLetterheads, cipher parentbilling.BankDataCipher, fingerprint func([]byte) string) parentbilling.SEPADebits {
	return application.NewSEPADebits(postgres.NewStore(), letterheads, cipher, calendar.TodayDate, calendar.Now, fingerprint)
}
