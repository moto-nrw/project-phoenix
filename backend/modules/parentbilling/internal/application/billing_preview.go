package application

import (
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The preview of a calculated run (ADR 0119): one draft invoice per
// confirmed registration of the period that no released run billed yet. Its
// positions are the lines of the contribution the confirmation froze, then
// the corrections of the run in their order. It is pure; the caller reads
// the registrations, the contributions, the billed registrations and the
// corrections.

// previewInput is what the preview of one run reads.
type previewInput struct {
	run           parentbilling.BillingRun
	registrations []parentbilling.HolidayBillingRegistration
	contributions map[int64]parentbilling.HolidayContribution
	issued        map[int64]bool
	corrections   []parentbilling.BillingCorrection
}

// preview is the draft invoices of a run and the confirmed registrations it
// leaves out because they have no frozen contribution.
type preview struct {
	documents []parentbilling.BillingDocument
	unbilled  int
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
	for _, registration := range registrations {
		if in.issued[registration.ID] {
			continue
		}
		contribution, ok := in.contributions[registration.ID]
		if !ok {
			result.unbilled++
			continue
		}
		document := draftDocument(in.run, registration, contribution, byChild)
		if len(document.Children) > 0 {
			result.documents = append(result.documents, document)
		}
	}
	return result
}

// draftDocument is the invoice of one registration: every child with a
// frozen line, in the order of the form.
func draftDocument(run parentbilling.BillingRun, registration parentbilling.HolidayBillingRegistration,
	contribution parentbilling.HolidayContribution, corrections map[int64][]parentbilling.BillingCorrection,
) parentbilling.BillingDocument {
	frozen := map[int64]parentbilling.HolidayChildContribution{}
	for _, child := range contribution.Children {
		frozen[child.ChildID] = child
	}
	document := parentbilling.BillingDocument{
		RegistrationID: registration.ID, RecipientFirstName: registration.GuardianFirstName,
		RecipientLastName: registration.GuardianLastName, RecipientEmail: registration.GuardianEmail,
		Children: []parentbilling.BillingChild{},
	}
	var from, until calendar.Date
	for _, child := range registration.Children {
		lines := frozen[child.ID]
		if len(lines.Lines) == 0 {
			continue
		}
		billed := parentbilling.BillingChild{
			ChildID: child.ID, FirstName: child.FirstName, LastName: child.LastName,
			Positions: make([]parentbilling.BillingPosition, 0, len(lines.Lines)), Corrections: corrections[child.ID],
			TotalCents: lines.TotalCents,
		}
		if billed.Corrections == nil {
			billed.Corrections = []parentbilling.BillingCorrection{}
		}
		for _, line := range lines.Lines {
			billed.Positions = append(billed.Positions, parentbilling.BillingPosition{
				Kind: line.Kind, AmountCents: line.AmountCents, WeekStartsOn: line.WeekStartsOn, WeekEndsOn: line.WeekEndsOn,
				TimeModelName: line.TimeModelName, ChildRank: line.ChildRank, DiscountBasisPoints: line.DiscountBasisPoints,
				DiscountCents: line.DiscountCents, Note: line.Note,
			})
			from, until = widen(from, until, line.WeekStartsOn, line.WeekEndsOn)
		}
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
