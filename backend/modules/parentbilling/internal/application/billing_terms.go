package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Actions of the trail of a family deviation.
const (
	familyBillingRead   = "read"
	familyBillingSave   = "save"
	familyBillingDelete = "delete"
)

// paymentTermsReader reads in one statement what decides the payment and
// the recipient of the invoices of a run: the terms of the Träger, the
// deviations of its families and the due dates of a monthly run
// (careRunID) or of a holiday run (holidayRunID); the other is zero.
type paymentTermsReader interface {
	InvoicePaymentTerms(ctx context.Context, organizationID, careRunID, holidayRunID int64) (
		[]parentbilling.ServiceTerms, []parentbilling.FamilyBilling, parentbilling.RunDueDates, error)
}

// TermsStore persists the terms, the deviations and the due dates.
type TermsStore interface {
	paymentTermsReader
	ServiceTerms(ctx context.Context, organizationID int64) ([]parentbilling.ServiceTerms, error)
	FamilyBillings(ctx context.Context, organizationID int64) ([]parentbilling.FamilyBilling, error)
	SaveServiceTerms(ctx context.Context, organizationID, accountID int64, terms parentbilling.ServiceTerms) error
	// SaveFamilyBilling inserts or replaces the deviation of its family.
	SaveFamilyBilling(ctx context.Context, organizationID, accountID int64, family parentbilling.FamilyBilling) (parentbilling.FamilyBilling, error)
	// DeleteFamilyBilling removes a deviation by its ID or, with id zero, by
	// its family; it answers the ID it removed, zero for none.
	DeleteFamilyBilling(ctx context.Context, organizationID, id int64, familyEmail string) (int64, error)
	InsertFamilyBillingEvents(ctx context.Context, organizationID, accountID int64, action string, ids []int64) error
	// ReplaceRunDueDates replaces the due dates of a run; the database
	// refuses it for a released run.
	ReplaceRunDueDates(ctx context.Context, organizationID, accountID, careRunID, holidayRunID int64, dates parentbilling.RunDueDates) error
	CareRun(ctx context.Context, organizationID, runID int64, lock bool) (parentbilling.CareRun, error)
	LockBillingRun(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error)
}

// BillingTerms serves parentbilling.BillingTerms. The family of a contract
// comes from the Betreuungsvertrag through the root-bound port.
type BillingTerms struct {
	store     TermsStore
	contracts parentbilling.CareContracts
}

var _ parentbilling.BillingTerms = (*BillingTerms)(nil)

// NewBillingTerms builds the terms.
func NewBillingTerms(store TermsStore, contracts parentbilling.CareContracts) *BillingTerms {
	if store == nil || contracts == nil {
		panic("parent billing: the store and the care contracts are required")
	}
	return &BillingTerms{store: store, contracts: contracts}
}

// ServiceTerms answers the terms of every service, the defaults where the
// Träger set none.
func (t *BillingTerms) ServiceTerms(ctx context.Context, organizationID int64) ([]parentbilling.ServiceTerms, error) {
	if organizationID <= 0 {
		return nil, errors.New("parent billing: the Träger is required")
	}
	stored, err := t.store.ServiceTerms(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the service terms: %w", err)
	}
	set := termsSetOf(stored)
	list := make([]parentbilling.ServiceTerms, 0, len(parentbilling.BillingServices))
	for _, service := range parentbilling.BillingServices {
		list = append(list, set.Of(service.Service))
	}
	return list, nil
}

// termsSetOf keys the stored terms of a Träger by service.
func termsSetOf(stored []parentbilling.ServiceTerms) parentbilling.TermsSet {
	set := parentbilling.TermsSet{}
	for _, terms := range stored {
		terms.Custom = true
		set[terms.Service] = terms
	}
	return set
}

// SaveServiceTerms validates and stores the terms of one service.
func (t *BillingTerms) SaveServiceTerms(ctx context.Context, reviewer parentbilling.HolidayReviewer, terms parentbilling.ServiceTerms) ([]parentbilling.ServiceTerms, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	normalized, err := normalizeServiceTerms(terms)
	if err != nil {
		return nil, err
	}
	if err := t.store.SaveServiceTerms(ctx, reviewer.OrganizationID, reviewer.AccountID, normalized); err != nil {
		return nil, err
	}
	return t.ServiceTerms(ctx, reviewer.OrganizationID)
}

func normalizeServiceTerms(terms parentbilling.ServiceTerms) (parentbilling.ServiceTerms, error) {
	index := slices.IndexFunc(parentbilling.BillingServices, func(s parentbilling.BillingService) bool { return s.Service == terms.Service })
	if index < 0 {
		return terms, parentbilling.ErrServiceUnknown
	}
	errs := validation.Errors{}
	if !slices.Contains(parentbilling.PaymentMethods, terms.PaymentMethod) {
		errs["payment_method"] = errors.New("must be transfer or direct_debit")
	}
	if terms.DueDays < 0 || terms.DueDays > parentbilling.MaxDueDays {
		errs["due_days"] = fmt.Errorf("must be between 0 and %d", parentbilling.MaxDueDays)
	}
	if terms.BundleWith != "" && !slices.Contains(parentbilling.BillingServices[index].BundleWith, terms.BundleWith) {
		errs["bundle_with"] = errors.New("this service cannot travel on that invoice")
	}
	if len(errs) > 0 {
		return terms, errs
	}
	return parentbilling.ServiceTerms{Service: terms.Service, PaymentMethod: terms.PaymentMethod, DueDays: terms.DueDays,
		BundleWith: terms.BundleWith}, nil
}

// ListFamilyBilling lists the deviations and records the read of each.
func (t *BillingTerms) ListFamilyBilling(ctx context.Context, reviewer parentbilling.HolidayReviewer) ([]parentbilling.FamilyBilling, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	families, err := t.store.FamilyBillings(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(families))
	for i, family := range families {
		ids[i] = family.ID
	}
	if err := t.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead, ids); err != nil {
		return nil, fmt.Errorf("parent billing: record the read of the family billing: %w", err)
	}
	return families, nil
}

// SaveFamilyBilling validates the deviation and stores it for the family of
// the contract. A deviation that deviates in nothing removes the stored one,
// so the family falls back to the registration and the terms.
func (t *BillingTerms) SaveFamilyBilling(ctx context.Context, reviewer parentbilling.HolidayReviewer, input parentbilling.FamilyBillingInput) (*parentbilling.FamilyBilling, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	family, err := normalizeFamilyBilling(input)
	if err != nil {
		return nil, err
	}
	if input.ContractID <= 0 {
		return nil, parentbilling.ErrFamilyUnknown
	}
	recipients, err := t.contracts.Recipients(ctx, reviewer.OrganizationID, []int64{input.ContractID})
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the contract party: %w", err)
	}
	key := parentbilling.FamilyKey(recipients[input.ContractID].Email)
	if key == "" {
		return nil, parentbilling.ErrFamilyUnknown
	}
	family.FamilyEmail, family.ContractID = key, input.ContractID
	if !deviates(family) {
		removed, err := t.store.DeleteFamilyBilling(ctx, reviewer.OrganizationID, 0, key)
		if err != nil {
			return nil, err
		}
		if removed > 0 {
			if err := t.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingDelete, []int64{removed}); err != nil {
				return nil, fmt.Errorf("parent billing: record the family billing: %w", err)
			}
		}
		return nil, nil
	}
	saved, err := t.store.SaveFamilyBilling(ctx, reviewer.OrganizationID, reviewer.AccountID, family)
	if err != nil {
		return nil, err
	}
	if err := t.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, []int64{saved.ID}); err != nil {
		return nil, fmt.Errorf("parent billing: record the family billing: %w", err)
	}
	return &saved, nil
}

// deviates tells whether a deviation changes anything.
func deviates(family parentbilling.FamilyBilling) bool {
	return family.RecipientKind != parentbilling.RecipientApplicant || family.PaymentMethod != "" || family.DueDay != nil
}

// DeleteFamilyBilling removes a deviation; a missing one is
// ErrFamilyBillingNotFound.
func (t *BillingTerms) DeleteFamilyBilling(ctx context.Context, reviewer parentbilling.HolidayReviewer, id int64) error {
	if err := validReviewer(reviewer); err != nil {
		return err
	}
	if id <= 0 {
		return parentbilling.ErrFamilyBillingNotFound
	}
	removed, err := t.store.DeleteFamilyBilling(ctx, reviewer.OrganizationID, id, "")
	if err != nil {
		return err
	}
	if removed == 0 {
		return parentbilling.ErrFamilyBillingNotFound
	}
	if err := t.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingDelete, []int64{removed}); err != nil {
		return fmt.Errorf("parent billing: record the family billing: %w", err)
	}
	return nil
}

var postalCodePattern = regexp.MustCompile(`^[0-9]{5}$`)

// normalizeFamilyBilling trims the input and checks that a kind brings the
// names it needs: both parents a second name, another payer a name, a mail
// address and a postal address.
func normalizeFamilyBilling(input parentbilling.FamilyBillingInput) (parentbilling.FamilyBilling, error) {
	family := parentbilling.FamilyBilling{RecipientKind: input.RecipientKind, PaymentMethod: input.PaymentMethod, DueDay: input.DueDay}
	errs := validation.Errors{}
	switch input.RecipientKind {
	case parentbilling.RecipientApplicant:
	case parentbilling.RecipientBoth:
		family.SecondFirstName, family.SecondLastName = strings.TrimSpace(input.SecondFirstName), strings.TrimSpace(input.SecondLastName)
		requireName(errs, "second_first_name", family.SecondFirstName)
		requireName(errs, "second_last_name", family.SecondLastName)
	case parentbilling.RecipientPayer:
		family.PayerFirstName, family.PayerLastName = strings.TrimSpace(input.PayerFirstName), strings.TrimSpace(input.PayerLastName)
		family.PayerEmail = strings.TrimSpace(input.PayerEmail)
		requireName(errs, "payer_first_name", family.PayerFirstName)
		requireName(errs, "payer_last_name", family.PayerLastName)
		if address, err := mail.ParseAddress(family.PayerEmail); err != nil || address.Address != family.PayerEmail || len(family.PayerEmail) > 320 {
			errs["payer_email"] = errors.New("must be a valid e-mail address")
		}
		family.PayerAddress = normalizeAddress(errs, input.PayerAddress)
	default:
		errs["recipient_kind"] = errors.New("must be applicant, both or payer")
	}
	if family.PaymentMethod != "" && !slices.Contains(parentbilling.PaymentMethods, family.PaymentMethod) {
		errs["payment_method"] = errors.New("must be transfer or direct_debit")
	}
	if family.DueDay != nil && (*family.DueDay < 1 || *family.DueDay > parentbilling.MaxDueDay) {
		errs["due_day"] = fmt.Errorf("must be between 1 and %d", parentbilling.MaxDueDay)
	}
	if len(errs) > 0 {
		return family, errs
	}
	return family, nil
}

func requireName(errs validation.Errors, field, value string) {
	switch {
	case value == "":
		errs[field] = errors.New("is required")
	case len([]rune(value)) > parentbilling.MaxBillingNameLength:
		errs[field] = fmt.Errorf("must be at most %d characters", parentbilling.MaxBillingNameLength)
	}
}

func normalizeAddress(errs validation.Errors, address *parentbilling.PostalAddress) *parentbilling.PostalAddress {
	if address == nil {
		errs["payer_address"] = errors.New("is required")
		return nil
	}
	normalized := &parentbilling.PostalAddress{Street: strings.TrimSpace(address.Street), PostalCode: strings.TrimSpace(address.PostalCode),
		City: strings.TrimSpace(address.City)}
	if normalized.Street == "" || len([]rune(normalized.Street)) > 100 {
		errs["payer_address.street"] = errors.New("is required, at most 100 characters")
	}
	if !postalCodePattern.MatchString(normalized.PostalCode) {
		errs["payer_address.postal_code"] = errors.New("must be five digits")
	}
	if normalized.City == "" || len([]rune(normalized.City)) > 100 {
		errs["payer_address.city"] = errors.New("is required, at most 100 characters")
	}
	return normalized
}

// SetCareRunDueDates replaces the due dates of a calculated monthly run.
func (t *BillingTerms) SetCareRunDueDates(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, dates parentbilling.RunDueDates) (parentbilling.RunDueDates, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	if err := validDueDates(dates); err != nil {
		return nil, err
	}
	if runID <= 0 {
		return nil, parentbilling.ErrCareRunNotFound
	}
	run, err := t.store.CareRun(ctx, reviewer.OrganizationID, runID, true)
	if err != nil {
		return nil, err
	}
	if run.Status != parentbilling.RunCalculated {
		return nil, parentbilling.ErrBillingRunReleased
	}
	if err := t.store.ReplaceRunDueDates(ctx, reviewer.OrganizationID, reviewer.AccountID, run.ID, 0, dates); err != nil {
		return nil, err
	}
	return dates, nil
}

// SetHolidayRunDueDates replaces the due dates of a calculated holiday run.
func (t *BillingTerms) SetHolidayRunDueDates(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, dates parentbilling.RunDueDates) (parentbilling.RunDueDates, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	if err := validDueDates(dates); err != nil {
		return nil, err
	}
	if runID <= 0 {
		return nil, parentbilling.ErrBillingRunNotFound
	}
	run, err := t.store.LockBillingRun(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != parentbilling.RunCalculated {
		return nil, parentbilling.ErrBillingRunReleased
	}
	if err := t.store.ReplaceRunDueDates(ctx, reviewer.OrganizationID, reviewer.AccountID, 0, run.ID, dates); err != nil {
		return nil, err
	}
	return dates, nil
}

func validDueDates(dates parentbilling.RunDueDates) error {
	errs := validation.Errors{}
	for method, due := range dates {
		if !slices.Contains(parentbilling.PaymentMethods, method) {
			errs[method] = errors.New("is no payment method")
		} else if due.IsZero() {
			errs[method] = errors.New("must be a date")
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// invoicePayments is what decides the payment and the recipient of the
// invoices of one run.
type invoicePayments struct {
	terms    parentbilling.TermsSet
	run      parentbilling.RunDueDates
	families map[string]*parentbilling.FamilyBilling
}

// readInvoicePayments reads the terms, the deviations and the dates of a run.
func readInvoicePayments(ctx context.Context, store paymentTermsReader, organizationID, careRunID, holidayRunID int64) (invoicePayments, error) {
	stored, families, dates, err := store.InvoicePaymentTerms(ctx, organizationID, careRunID, holidayRunID)
	if err != nil {
		return invoicePayments{}, fmt.Errorf("parent billing: read the payment terms: %w", err)
	}
	if dates == nil {
		dates = parentbilling.RunDueDates{}
	}
	byFamily := make(map[string]*parentbilling.FamilyBilling, len(families))
	for i := range families {
		byFamily[families[i].FamilyEmail] = &families[i]
	}
	return invoicePayments{terms: termsSetOf(stored), run: dates, families: byFamily}, nil
}

// careInvoice sets the recipient and the payment of a monthly invoice.
// The invoice carries the care and every service bundled with it.
func (p invoicePayments) careInvoice(invoice *parentbilling.CareInvoice, issuedOn calendar.Date) {
	family := p.families[invoice.FamilyEmail]
	recipient := parentbilling.ResolveRecipient(parentbilling.BillingRecipient{FirstName: invoice.Recipient.FirstName,
		LastName: invoice.Recipient.LastName, Email: invoice.Recipient.Email}, family)
	invoice.Recipient = parentbilling.CareRecipient{FirstName: recipient.FirstName, LastName: recipient.LastName,
		Email: recipient.Email, Kind: recipient.Kind, SecondName: recipient.SecondName, Address: recipient.Address}
	payment := parentbilling.ResolvePayment(p.terms.Of(parentbilling.ServiceCare), p.run, family, issuedOn)
	invoice.Payment = &payment
}

// holidayDocument sets the recipient and the payment of a holiday invoice;
// the registration's address stays for both parents.
func (p invoicePayments) holidayDocument(document *parentbilling.BillingDocument, issuedOn calendar.Date) {
	document.FamilyEmail = parentbilling.FamilyKey(document.RecipientEmail)
	family := p.families[document.FamilyEmail]
	recipient := parentbilling.ResolveRecipient(parentbilling.BillingRecipient{FirstName: document.RecipientFirstName,
		LastName: document.RecipientLastName, Email: document.RecipientEmail, Address: document.RecipientAddress}, family)
	document.RecipientFirstName, document.RecipientLastName, document.RecipientEmail = recipient.FirstName, recipient.LastName, recipient.Email
	document.RecipientAddress, document.RecipientKind, document.RecipientSecondName = recipient.Address, recipient.Kind, recipient.SecondName
	payment := parentbilling.ResolvePayment(p.terms.Of(parentbilling.ServiceHoliday), p.run, family, issuedOn)
	document.Payment = &payment
}
