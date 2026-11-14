package application

import (
	"sort"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The preview of a calculated run (ADR 0119): one draft invoice per
// confirmed registration of the period that no released run billed yet, or
// one per child of it when the Träger keeps the debtor number per child
// (#7007). A registration keeps the shape of its first invoice: a change of
// the setting splits or joins only registrations without one. Its
// positions are the lines of the contribution the confirmation froze, then
// the corrections of the cancelled invoice it replaces (#5663), then the
// corrections of the run in their order. It is pure; the caller reads the
// registrations, the contributions, the billed registrations and the
// corrections.

// previewInput is what the preview of one run reads.
type previewInput struct {
	run           parentbilling.BillingRun
	registrations []parentbilling.HolidayBillingRegistration
	contributions map[int64]parentbilling.HolidayContribution
	issued        map[parentbilling.HolidayInvoiceKey]bool
	corrections   []parentbilling.BillingCorrection
	// replaces names the cancelled invoice a draft replaces (#4690).
	replaces map[parentbilling.HolidayInvoiceKey]parentbilling.BillingDocumentLink
	// replaced is what the draft takes from that invoice (#5663).
	replaced replacedInvoices
	// perChild bills each child of a registration without an invoice on
	// an invoice of its own (#7007).
	perChild bool
}

// invoiceShapes says for each registration with an invoice, issued or
// cancelled, whether its invoices bill its children one by one.
func (in previewInput) invoiceShapes() map[int64]bool {
	shapes := map[int64]bool{}
	for key := range in.issued {
		shapes[key.RegistrationID] = key.ChildID != 0
	}
	for key := range in.replaces {
		shapes[key.RegistrationID] = key.ChildID != 0
	}
	return shapes
}

// invoiceKeys are the invoices a registration needs: one of the whole
// registration or one per child.
func (in previewInput) invoiceKeys(registration parentbilling.HolidayBillingRegistration, shapes map[int64]bool) []parentbilling.HolidayInvoiceKey {
	perChild, known := shapes[registration.ID]
	if !known {
		perChild = in.perChild
	}
	if !perChild {
		return []parentbilling.HolidayInvoiceKey{{RegistrationID: registration.ID}}
	}
	keys := make([]parentbilling.HolidayInvoiceKey, len(registration.Children))
	for i, child := range registration.Children {
		keys[i] = parentbilling.HolidayInvoiceKey{RegistrationID: registration.ID, ChildID: child.ID}
	}
	return keys
}

// preview is the draft invoices of a run and the confirmed registrations it
// leaves out because they have no frozen contribution.
type preview struct {
	documents []parentbilling.BillingDocument
	unbilled  int
	// dueDates are the run's due dates the payments used (#4689).
	dueDates parentbilling.RunDueDates
}

func (p preview) total() int64 {
	var total int64
	for _, document := range p.documents {
		total += document.TotalCents
	}
	return total
}

// child finds the child of a registration in the preview.
func (p preview) child(registrationID, childID int64) (parentbilling.BillingChild, bool) {
	for _, document := range p.documents {
		if document.RegistrationID != registrationID {
			continue
		}
		for _, child := range document.Children {
			if child.ChildID == childID {
				return child, true
			}
		}
	}
	return parentbilling.BillingChild{}, false
}

func buildPreview(in previewInput) preview {
	registrations := append([]parentbilling.HolidayBillingRegistration(nil), in.registrations...)
	sort.Slice(registrations, func(i, j int) bool { return registrations[i].ID < registrations[j].ID })
	byChild := map[int64][]parentbilling.BillingCorrection{}
	for _, correction := range in.corrections {
		byChild[correction.ChildID] = append(byChild[correction.ChildID], correction)
	}
	result := preview{documents: []parentbilling.BillingDocument{}}
	shapes := in.invoiceShapes()
	for _, registration := range registrations {
		open := in.openInvoices(registration, shapes)
		if len(open) == 0 {
			continue
		}
		contribution, ok := in.contributions[registration.ID]
		if !ok {
			result.unbilled++
			continue
		}
		for _, key := range open {
			if document := in.draft(registration, contribution, key, byChild); len(document.Children) > 0 {
				result.documents = append(result.documents, document)
			}
		}
	}
	return result
}

// openInvoices are the invoices of the registration no issued invoice is
// yet.
func (in previewInput) openInvoices(registration parentbilling.HolidayBillingRegistration, shapes map[int64]bool) []parentbilling.HolidayInvoiceKey {
	var open []parentbilling.HolidayInvoiceKey
	for _, key := range in.invoiceKeys(registration, shapes) {
		if !in.issued[key] {
			open = append(open, key)
		}
	}
	return open
}

// draft is the draft of one invoice of the registration, with the cancelled
// invoice it replaces.
func (in previewInput) draft(registration parentbilling.HolidayBillingRegistration, contribution parentbilling.HolidayContribution,
	key parentbilling.HolidayInvoiceKey, byChild map[int64][]parentbilling.BillingCorrection,
) parentbilling.BillingDocument {
	document := draftDocument(in.run, registration, contribution, key.ChildID, in.replaced.corrections[key], byChild)
	if cancelled, ok := in.replaces[key]; ok {
		document.ReferenceDocumentID, document.ReferenceNumber = &cancelled.ID, cancelled.Number
		document.ReplacedCredits = in.replaced.credits[key]
	}
	if document.ReplacedCredits == nil {
		document.ReplacedCredits = []parentbilling.BillingDocumentLink{}
	}
	return document
}

// draftDocument is the invoice of one registration: every child with a
// frozen line, in the order of the form, or only the child childID names
// (#7007). An invoice of one child of several takes the registration's
// debtor number only as the number of that child.
func draftDocument(run parentbilling.BillingRun, registration parentbilling.HolidayBillingRegistration,
	contribution parentbilling.HolidayContribution, childID int64, carried []parentbilling.BillingCarriedCorrection,
	corrections map[int64][]parentbilling.BillingCorrection,
) parentbilling.BillingDocument {
	frozen := map[int64]parentbilling.HolidayChildContribution{}
	for _, child := range contribution.Children {
		frozen[child.ChildID] = child
	}
	document := parentbilling.BillingDocument{
		Kind: parentbilling.DocumentInvoice, RegistrationID: registration.ID, RecipientFirstName: registration.GuardianFirstName,
		RecipientLastName: registration.GuardianLastName, RecipientEmail: registration.GuardianEmail,
		RecipientAddress: registration.GuardianAddress, DebtorNumber: draftDebtorNumber(registration, childID),
		RegistrationChildID: childID, Children: []parentbilling.BillingChild{},
	}
	carriedByChild := map[int64][]parentbilling.BillingCarriedCorrection{}
	for _, correction := range carried {
		carriedByChild[correction.ChildID] = append(carriedByChild[correction.ChildID], correction)
	}
	var from, until calendar.Date
	for _, child := range registration.Children {
		lines := frozen[child.ID]
		if len(lines.Lines) == 0 || !billsChild(childID, child.ID) {
			continue
		}
		billed := parentbilling.BillingChild{
			ChildID: child.ID, FirstName: child.FirstName, LastName: child.LastName, BirthDate: child.BirthDate,
			Positions: make([]parentbilling.BillingPosition, 0, len(lines.Lines)), Corrections: corrections[child.ID],
			TotalCents: lines.TotalCents,
		}
		if billed.Corrections == nil {
			billed.Corrections = []parentbilling.BillingCorrection{}
		}
		for _, line := range lines.Lines {
			billed.Positions = append(billed.Positions, billingPosition(line))
			from, until = widen(from, until, line.WeekStartsOn, line.WeekEndsOn)
		}
		billed = withCarriedCorrections(billed, carriedByChild[child.ID])
		for _, correction := range billed.Corrections {
			billed.Positions = append(billed.Positions, parentbilling.BillingPosition{
				Kind: parentbilling.LineCorrection, AmountCents: correction.AmountCents, Note: correction.Reason,
			})
			billed.TotalCents += correction.AmountCents
		}
		document.Children = append(document.Children, billed)
		document.TotalCents += billed.TotalCents
	}
	document.ServiceFrom, document.ServiceUntil = run.ServiceFrom, run.ServiceUntil
	if !from.IsZero() {
		document.ServiceFrom, document.ServiceUntil = from, until
	}
	return document
}

// draftDebtorNumber is the registration's number on its invoice: on an
// invoice of one child of several it names no child and stays off.
func draftDebtorNumber(registration parentbilling.HolidayBillingRegistration, childID int64) string {
	if childID != 0 && len(registration.Children) > 1 {
		return ""
	}
	return registration.DebtorNumber
}

// billsChild says whether the invoice of childID, zero for the whole
// registration, bills the child.
func billsChild(childID, child int64) bool {
	return childID == 0 || childID == child
}

// widen extends the service days of an invoice by the week of a line.
func widen(from, until calendar.Date, starts, ends *calendar.Date) (calendar.Date, calendar.Date) {
	if starts != nil && (from.IsZero() || starts.Before(from)) {
		from = *starts
	}
	if ends != nil && (until.IsZero() || ends.After(until)) {
		until = *ends
	}
	return from, until
}

// billingPosition is a line of the frozen contribution as a position of the
// invoice; the week it prices names its host school at the release (#6624).
func billingPosition(line parentbilling.HolidayContributionLine) parentbilling.BillingPosition {
	return parentbilling.BillingPosition{
		Kind: line.Kind, AmountCents: line.AmountCents, WeekStartsOn: line.WeekStartsOn, WeekEndsOn: line.WeekEndsOn,
		TimeModelName: line.TimeModelName, ChildRank: line.ChildRank, DiscountBasisPoints: line.DiscountBasisPoints,
		DiscountCents: line.DiscountCents, Note: line.Note, CostBearerID: line.CostBearerID, CostBearerName: line.CostBearerName,
		WeekID: line.WeekID,
	}
}

// hasChildInvoices includes replacements that keep their child-specific
// shape after a switch to payer scope.
func hasChildInvoices(documents []parentbilling.BillingDocument) bool {
	for _, document := range documents {
		if document.RegistrationChildID != 0 {
			return true
		}
	}
	return false
}
