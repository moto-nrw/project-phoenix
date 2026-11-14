package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// AmendCareInvoice appends one change of an invoice of a calculated run
// (#6197). It holds the run like the release, checks that the Träger allows
// the kind and that the review is the one the change was made on, and
// checks the change against the invoice as the review shows it now.
func (c *CareBilling) AmendCareInvoice(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, input parentbilling.CareAmendmentInput) (parentbilling.CareRunDetail, error) {
	if err := checkAmendmentInput(&input); err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	run, err := c.openRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	amendable, err := c.settings.CareAmendable(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: read the changes the Träger allows: %w", err)
	}
	if !amendable.Allows(input.Kind) {
		return parentbilling.CareRunDetail{}, parentbilling.ErrCareAmendmentDisabled
	}
	_, review, err := c.reviewed(ctx, reviewer.OrganizationID, run)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	if input.Fingerprint != review.fingerprint {
		return parentbilling.CareRunDetail{}, parentbilling.ErrBillingRunChanged
	}
	key := parentbilling.CareInvoiceKey{ContractID: input.ContractID, Service: input.Service}
	all := slices.Concat(review.invoices, review.excluded)
	index := slices.IndexFunc(all, func(invoice parentbilling.CareInvoice) bool { return careInvoiceKeyOf(invoice) == key })
	if index < 0 {
		return parentbilling.CareRunDetail{}, parentbilling.ErrCareAmendmentConflict
	}
	entry, err := amendmentEntry(all[index], input)
	if err != nil {
		return parentbilling.CareRunDetail{}, err
	}
	entry.Invoice, entry.AccountID = key, &reviewer.AccountID
	if err := c.store.InsertCareRunAmendment(ctx, reviewer.OrganizationID, run.ID, entry); err != nil {
		return parentbilling.CareRunDetail{}, fmt.Errorf("parent billing: record the change: %w", err)
	}
	return c.CareRun(ctx, reviewer, run.ID)
}

// checkAmendmentInput checks what a change needs whatever its invoice: a
// known kind, an invoice, a service, the fingerprint and a reason.
func checkAmendmentInput(input *parentbilling.CareAmendmentInput) error {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Service == "" {
		input.Service = parentbilling.ServiceCare
	}
	errs := validation.Errors{}
	switch input.Kind {
	case parentbilling.AmendAmount, parentbilling.AmendAddPosition, parentbilling.AmendRemovePosition,
		parentbilling.AmendRecipient, parentbilling.AmendAddress, parentbilling.AmendRevoke:
	default:
		errs["kind"] = errors.New("must be amount, add_position, remove_position, recipient, address or revoke")
	}
	if input.ContractID <= 0 {
		errs["contract_id"] = errors.New("is required")
	}
	if input.Service != parentbilling.ServiceCare && input.Service != parentbilling.ServiceMeal {
		errs["service"] = errors.New("must be care or meal")
	}
	if strings.TrimSpace(input.Fingerprint) == "" {
		errs["fingerprint"] = errors.New("is required")
	}
	switch {
	case input.Reason == "":
		errs["reason"] = errors.New("is required")
	case len([]rune(input.Reason)) > parentbilling.MaxAmendmentReasonLength:
		errs["reason"] = fmt.Errorf("must be at most %d characters", parentbilling.MaxAmendmentReasonLength)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// amendmentEntry builds the entry of a change from the invoice as the
// review shows it: the old value is what the invoice shows, the new value
// what the change asks for.
func amendmentEntry(invoice parentbilling.CareInvoice, input parentbilling.CareAmendmentInput) (parentbilling.CareAmendmentEntry, error) {
	entry := parentbilling.CareAmendmentEntry{Kind: input.Kind, Reason: input.Reason}
	var err error
	switch input.Kind {
	case parentbilling.AmendAmount:
		err = amountEntry(&entry, invoice, input.TotalCents)
	case parentbilling.AmendAddPosition:
		err = addedPositionEntry(&entry, input.Label, input.AmountCents)
	case parentbilling.AmendRemovePosition:
		err = removedPositionEntry(&entry, invoice, input.PositionKey)
	case parentbilling.AmendRecipient:
		err = recipientEntry(&entry, invoice, input.Recipient)
	case parentbilling.AmendAddress:
		err = addressEntry(&entry, invoice, input.Address)
	case parentbilling.AmendRevoke:
		err = revocationEntry(&entry, invoice, input.RevokesID, input.RevokesCarried)
	}
	return entry, err
}

func amountEntry(entry *parentbilling.CareAmendmentEntry, invoice parentbilling.CareInvoice, total *int64) error {
	switch {
	case total == nil:
		return validation.Errors{"total_cents": errors.New("is required")}
	case *total < 0 || *total > parentbilling.MaxMonthlyPriceCents:
		return validation.Errors{"total_cents": fmt.Errorf("must be between 0 and %d", parentbilling.MaxMonthlyPriceCents)}
	case *total == invoice.TotalCents:
		return validation.Errors{"total_cents": errors.New("must differ from the total of the invoice")}
	}
	before, after := invoice.TotalCents, *total
	entry.Label, entry.AmountCents = amendAmountLabel, after-before
	entry.Before.TotalCents, entry.After.TotalCents = &before, &after
	return nil
}

func addedPositionEntry(entry *parentbilling.CareAmendmentEntry, label string, amount *int64) error {
	label = strings.TrimSpace(label)
	errs := validation.Errors{}
	if label == "" || len([]rune(label)) > parentbilling.MaxAmendmentLabelLength {
		errs["label"] = fmt.Errorf("is required, at most %d characters", parentbilling.MaxAmendmentLabelLength)
	}
	if amount == nil || *amount <= 0 || *amount > parentbilling.MaxMonthlyPriceCents {
		errs["amount_cents"] = fmt.Errorf("must be between 1 and %d", parentbilling.MaxMonthlyPriceCents)
	}
	if len(errs) > 0 {
		return errs
	}
	entry.Label, entry.AmountCents = label, *amount
	entry.After.Label, entry.After.AmountCents = label, amount
	return nil
}

func removedPositionEntry(entry *parentbilling.CareAmendmentEntry, invoice parentbilling.CareInvoice, key string) error {
	label, cents, ok := removablePosition(invoice, key)
	if !ok || slices.ContainsFunc(invoice.Amendments, func(amendment parentbilling.CareAmendment) bool {
		return amendment.Applied() && amendment.Kind == parentbilling.AmendRemovePosition && amendment.PositionKey == key
	}) {
		return parentbilling.ErrCareAmendmentConflict
	}
	if invoice.TotalCents-cents < 0 {
		return validation.Errors{"position_key": errors.New("would take the invoice below zero")}
	}
	entry.Label, entry.AmountCents, entry.PositionKey = amendRemovePrefix+label, -cents, key
	entry.Before.Label, entry.Before.AmountCents = label, &cents
	return nil
}

func recipientEntry(entry *parentbilling.CareAmendmentEntry, invoice parentbilling.CareInvoice, name *parentbilling.CareRecipientName) error {
	if name == nil {
		return validation.Errors{"recipient": errors.New("is required")}
	}
	after := parentbilling.CareRecipientName{FirstName: strings.TrimSpace(name.FirstName), LastName: strings.TrimSpace(name.LastName),
		SecondName: strings.TrimSpace(name.SecondName), Email: strings.TrimSpace(name.Email)}
	errs := validation.Errors{}
	requireName(errs, "recipient.first_name", after.FirstName)
	requireName(errs, "recipient.last_name", after.LastName)
	if len([]rune(after.SecondName)) > 2*parentbilling.MaxBillingNameLength {
		errs["recipient.second_name"] = fmt.Errorf("must be at most %d characters", 2*parentbilling.MaxBillingNameLength)
	}
	if address, err := mail.ParseAddress(after.Email); err != nil || address.Address != after.Email || len(after.Email) > 320 {
		errs["recipient.email"] = errors.New("must be a valid e-mail address")
	}
	before := parentbilling.CareRecipientName{FirstName: invoice.Recipient.FirstName, LastName: invoice.Recipient.LastName,
		SecondName: invoice.Recipient.SecondName, Email: invoice.Recipient.Email}
	if len(errs) == 0 && after == before {
		errs["recipient"] = errors.New("must differ from the recipient of the invoice")
	}
	if len(errs) > 0 {
		return errs
	}
	entry.Before.Recipient, entry.After.Recipient = &before, &after
	return nil
}

func addressEntry(entry *parentbilling.CareAmendmentEntry, invoice parentbilling.CareInvoice, address *parentbilling.PostalAddress) error {
	errs := validation.Errors{}
	after := normalizeAddress(errs, "address", address)
	if len(errs) == 0 && invoice.Recipient.Address != nil && *invoice.Recipient.Address == *after {
		errs["address"] = errors.New("must differ from the address of the invoice")
	}
	if len(errs) > 0 {
		return errs
	}
	entry.Before.Address, entry.After.Address = invoice.Recipient.Address, after
	return nil
}

// revocationEntry takes back a change of the run or a carried correction
// that is in force on the invoice; a conflicting one may be taken back too.
func revocationEntry(entry *parentbilling.CareAmendmentEntry, invoice parentbilling.CareInvoice, id int64, carried bool) error {
	index := slices.IndexFunc(invoice.Amendments, func(amendment parentbilling.CareAmendment) bool {
		return amendment.ID == id && amendment.Carried == carried && amendment.Revocation == nil
	})
	if id <= 0 || index < 0 {
		return parentbilling.ErrCareAmendmentConflict
	}
	revoked := invoice.Amendments[index]
	if carried {
		entry.RevokesCorrectionID = &id
	} else {
		entry.RevokesID = &id
	}
	entry.Before = revoked.After
	return nil
}
