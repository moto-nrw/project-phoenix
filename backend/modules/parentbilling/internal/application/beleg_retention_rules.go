package application

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BelegDecision is the retention record of one Beleg on a day.
type BelegDecision struct {
	State         string
	Reason        string
	TriggerEvent  string
	TriggerDate   *calendar.Date
	DeletableOn   *calendar.Date
	ProposedOn    *calendar.Date
	InstructionID int64
	SuspensionID  int64
}

// Due reports whether the Beleg may be deleted on today: only with an
// instruction, without a suspension and from its first deletable day on.
func (d BelegDecision) Due(today calendar.Date) bool {
	return d.State == parentbilling.RetentionScheduled && d.DeletableOn != nil && !today.Before(*d.DeletableOn)
}

// triggersByType lists the events an instruction may name per type.
var triggersByType = map[string][]string{
	parentbilling.BelegHolidayDocument: {parentbilling.TriggerIssued},
	parentbilling.BelegCareInvoice:     {parentbilling.TriggerIssued},
	parentbilling.BelegCareNotice:      {parentbilling.TriggerIssued},
	parentbilling.BelegCareContract:    {parentbilling.TriggerContractEnded, parentbilling.TriggerContractConcluded},
	parentbilling.BelegSEPAMandate:     {parentbilling.TriggerLastCollection},
	parentbilling.BelegSEPADebit:       {parentbilling.TriggerIssued},
}

// basesByType lists the bases a Weisung may name per type: invoices,
// credit and cancellation notes and debits are booking vouchers, contracts
// and notices are classified, a mandate follows the Inkassovereinbarung
// (Löschkonzept section 2).
var basesByType = map[string][]string{
	parentbilling.BelegHolidayDocument: {parentbilling.BasisBookingVoucher, parentbilling.BasisNotApplicable},
	parentbilling.BelegCareInvoice:     {parentbilling.BasisBookingVoucher, parentbilling.BasisNotApplicable},
	parentbilling.BelegSEPADebit:       {parentbilling.BasisBookingVoucher, parentbilling.BasisNotApplicable},
	parentbilling.BelegCareNotice:      {parentbilling.BasisBookingVoucher, parentbilling.BasisBusinessLetter, parentbilling.BasisNotApplicable},
	parentbilling.BelegCareContract:    {parentbilling.BasisBookingVoucher, parentbilling.BasisBusinessLetter, parentbilling.BasisNotApplicable},
	parentbilling.BelegSEPAMandate:     {parentbilling.BasisCollectionAgreement},
}

// statutoryYears is the least period a basis allows.
var statutoryYears = map[string]int{
	parentbilling.BasisBookingVoucher: parentbilling.MinBookingVoucherYears,
	parentbilling.BasisBusinessLetter: parentbilling.MinBusinessLetterYears,
}

// bookingTypes have the period the Löschkonzept proposes; contracts and
// notices have none, the Träger classifies them.
var bookingTypes = []string{parentbilling.BelegHolidayDocument, parentbilling.BelegCareInvoice, parentbilling.BelegSEPADebit}

// DecideBelegRetention computes the record of one Beleg from its facts, the
// instruction of its type and an active suspension, both optional.
func DecideBelegRetention(fact parentbilling.BelegFact, instruction *parentbilling.RetentionInstruction, suspension *parentbilling.RetentionSuspension, today calendar.Date) BelegDecision {
	if instruction == nil {
		return proposedDecision(fact)
	}
	decision := BelegDecision{TriggerEvent: instruction.TriggerEvent, InstructionID: instruction.ID}
	if fact.BelegType == parentbilling.BelegSEPAMandate {
		decideMandate(&decision, fact, instruction.Period, today)
	} else {
		decideYearEnd(&decision, fact, instruction, today)
	}
	if suspension != nil {
		decision.State = parentbilling.RetentionSuspended
		decision.SuspensionID = suspension.ID
	}
	return decision
}

// proposedDecision lists a Beleg without instruction for clarification and
// shows a booking Beleg its proposed eight years.
func proposedDecision(fact parentbilling.BelegFact) BelegDecision {
	decision := BelegDecision{State: parentbilling.RetentionClarification, Reason: parentbilling.ClarifyNoInstruction}
	if slices.Contains(bookingTypes, fact.BelegType) && fact.Issued != nil {
		decision.TriggerEvent, decision.TriggerDate = parentbilling.TriggerIssued, fact.Issued
		decision.ProposedOn = afterYearEnd(*fact.Issued, parentbilling.ProposedBookingRetentionYears)
	}
	return decision
}

func decideYearEnd(decision *BelegDecision, fact parentbilling.BelegFact, instruction *parentbilling.RetentionInstruction, today calendar.Date) {
	// A contract stays available while it is in force, whatever event its
	// period counts from (Löschkonzept section 2).
	if fact.BelegType == parentbilling.BelegCareContract && (!fact.Ended || (fact.EndedOn != nil && fact.EndedOn.After(today))) {
		decision.State = parentbilling.RetentionRunning
		return
	}
	var event *calendar.Date
	switch instruction.TriggerEvent {
	case parentbilling.TriggerContractEnded:
		event = fact.EndedOn
	case parentbilling.TriggerContractConcluded:
		event = fact.ConcludedOn
	default:
		event = fact.Issued
	}
	if event == nil {
		decision.State, decision.Reason = parentbilling.RetentionClarification, parentbilling.ClarifyNoTriggerDate
		return
	}
	decision.State = parentbilling.RetentionScheduled
	decision.TriggerDate = event
	decision.DeletableOn = afterYearEnd(*event, instruction.Period)
}

// decideMandate keeps a mandate while it is valid and, after its expiry, at
// least months from the submission of its last collected debit. A mandate
// that expired without a collection gets no invented date.
func decideMandate(decision *BelegDecision, fact parentbilling.BelegFact, months int, today calendar.Date) {
	if !mandateExpired(fact, today) {
		decision.State = parentbilling.RetentionRunning
		return
	}
	if fact.LastCollection == nil {
		decision.State, decision.Reason = parentbilling.RetentionClarification, parentbilling.ClarifyNoCollection
		return
	}
	end := addMonths(*fact.LastCollection, months)
	decision.State = parentbilling.RetentionScheduled
	decision.TriggerDate = fact.LastCollection
	decision.DeletableOn = &end
}

// mandateExpired: revoked, or 36 months without a collection since the last
// one or, never used, since its signature.
func mandateExpired(fact parentbilling.BelegFact, today calendar.Date) bool {
	if fact.RevokedOn != nil && !fact.RevokedOn.After(today) {
		return true
	}
	last := fact.LastCollection
	if last == nil {
		last = fact.SignedOn
	}
	return last != nil && !today.Before(addMonths(*last, parentbilling.MandateExpiryMonths))
}

// afterYearEnd is the first day after years full years following the end
// of the event's year: 2026 plus eight years is 1 January 2035.
func afterYearEnd(event calendar.Date, years int) *calendar.Date {
	day := calendar.NewDate(event.Year()+years+1, time.January, 1)
	return &day
}

// addMonths adds months to a day; a day the target month lacks rolls into
// the next month, so the period is never shorter than asked.
func addMonths(day calendar.Date, months int) calendar.Date {
	return calendar.NewDate(day.Year(), day.Month()+time.Month(months), day.Day())
}

// ValidateRetentionInstruction checks an instruction against its type.
func ValidateRetentionInstruction(instruction parentbilling.NewRetentionInstruction) error {
	triggers, known := triggersByType[instruction.BelegType]
	if !known {
		return fmt.Errorf("%w: unknown beleg type %q", parentbilling.ErrBelegRetentionInvalid, instruction.BelegType)
	}
	if !slices.Contains(triggers, instruction.TriggerEvent) {
		return fmt.Errorf("%w: event %q does not start the period of %s", parentbilling.ErrBelegRetentionInvalid, instruction.TriggerEvent, instruction.BelegType)
	}
	minimum, maximum := parentbilling.MinRetentionYears, parentbilling.MaxRetentionYears
	if instruction.BelegType == parentbilling.BelegSEPAMandate {
		minimum, maximum = parentbilling.MinMandateRetentionMonths, parentbilling.MaxMandateRetentionMonths
	}
	if instruction.Period < minimum || instruction.Period > maximum {
		return fmt.Errorf("%w: period %d outside %d to %d", parentbilling.ErrBelegRetentionInvalid, instruction.Period, minimum, maximum)
	}
	if instruction.InstructedOn.IsZero() {
		return fmt.Errorf("%w: the day of the instruction is required", parentbilling.ErrBelegRetentionInvalid)
	}
	if err := validateBasis(instruction); err != nil {
		return err
	}
	return requireTexts(map[string]string{"legal basis": instruction.LegalBasis, "instructed by": instruction.InstructedBy, "instruction reference": instruction.InstructionReference})
}

// validateBasis holds a Weisung to the statutes it names: a booking voucher
// keeps eight years, a business letter six. Only the stated basis that the
// statutes do not apply allows a shorter period, with its written
// justification and the explicit confirmation that the Träger confirmed in
// writing that § 147 AO, § 257 HGB and § 14b UStG do not apply; the
// instructing person and the reference to the written instruction are
// required of every Weisung. The text of the basis is no gate here: the
// database alone runs its citation heuristic (ErrBelegBasisCitesStatute).
func validateBasis(instruction parentbilling.NewRetentionInstruction) error {
	if !slices.Contains(basesByType[instruction.BelegType], instruction.Basis) {
		return fmt.Errorf("%w: basis %q does not fit %s", parentbilling.ErrBelegRetentionInvalid, instruction.Basis, instruction.BelegType)
	}
	if minimum, statutory := statutoryYears[instruction.Basis]; statutory && instruction.Period < minimum {
		return fmt.Errorf("%w: a %s is kept at least %d years", parentbilling.ErrBelegRetentionInvalid, instruction.Basis, minimum)
	}
	justification := strings.TrimSpace(instruction.BasisJustification)
	if instruction.Basis != parentbilling.BasisNotApplicable {
		if justification != "" || instruction.StatutesNotApplicableConfirmed {
			return fmt.Errorf("%w: a justification and the confirmation belong only to a basis without statutes", parentbilling.ErrBelegRetentionInvalid)
		}
		return nil
	}
	if justification == "" || len([]rune(justification)) > parentbilling.MaxJustificationLength {
		return fmt.Errorf("%w: why the statutes do not apply is required and at most %d characters", parentbilling.ErrBelegRetentionInvalid, parentbilling.MaxJustificationLength)
	}
	if !instruction.StatutesNotApplicableConfirmed {
		return fmt.Errorf("%w: the Träger's written confirmation that § 147 AO, § 257 HGB and § 14b UStG do not apply is required", parentbilling.ErrBelegRetentionInvalid)
	}
	return nil
}

// ValidateClarificationReview checks a review: a known type and reason, a
// note, the reason of an escalation and a next review after today, within
// a year.
func ValidateClarificationReview(review parentbilling.NewBelegClarificationReview, today calendar.Date) error {
	if _, known := triggersByType[review.BelegType]; !known {
		return fmt.Errorf("%w: unknown beleg type %q", parentbilling.ErrBelegRetentionInvalid, review.BelegType)
	}
	if !slices.Contains(clarificationReasons, review.Reason) {
		return fmt.Errorf("%w: unknown clarification reason %q", parentbilling.ErrBelegRetentionInvalid, review.Reason)
	}
	if !review.NextReviewOn.After(today) || review.NextReviewOn.After(today.AddDays(parentbilling.MaxClarificationReviewDays)) {
		return fmt.Errorf("%w: the next review lies after today and within %d days", parentbilling.ErrBelegRetentionInvalid, parentbilling.MaxClarificationReviewDays)
	}
	fields := map[string]string{"note": review.Note}
	if review.Escalated {
		fields["escalation reason"] = review.EscalationReason
	} else if strings.TrimSpace(review.EscalationReason) != "" {
		return fmt.Errorf("%w: an escalation reason needs the escalation", parentbilling.ErrBelegRetentionInvalid)
	}
	return requireLength(fields, parentbilling.MaxJustificationLength)
}

// ValidateClarificationEscalation holds a review to the deadline of its
// entry (Löschkonzept section 3): an entry still open at its review goes to
// Florian with a reason before its review date moves. Without escalation a
// review keeps or advances the date; an overdue entry, whose next review
// always lies after its review date, cannot drop its overdue status by a
// new date alone.
func ValidateClarificationEscalation(review parentbilling.NewBelegClarificationReview, entryReviewOn calendar.Date) error {
	if !review.Escalated && review.NextReviewOn.After(entryReviewOn) {
		return fmt.Errorf("%w: an entry still open at its review %s goes to Florian with a reason before its review moves",
			parentbilling.ErrBelegRetentionInvalid, entryReviewOn)
	}
	return nil
}

var clarificationReasons = []string{parentbilling.ClarifyNoInstruction, parentbilling.ClarifyNoCollection, parentbilling.ClarifyNoTriggerDate}

// ValidateRetentionSuspension checks a suspension: a known type, a reason,
// a scope, a responsible person and the next review.
func ValidateRetentionSuspension(suspension parentbilling.NewRetentionSuspension, today calendar.Date) error {
	if _, known := triggersByType[suspension.BelegType]; !known {
		return fmt.Errorf("%w: unknown beleg type %q", parentbilling.ErrBelegRetentionInvalid, suspension.BelegType)
	}
	if suspension.BelegID < 0 {
		return fmt.Errorf("%w: invalid beleg", parentbilling.ErrBelegRetentionInvalid)
	}
	if suspension.ReviewOn.IsZero() || suspension.ReviewOn.Before(today) {
		return fmt.Errorf("%w: the next review must be today or later", parentbilling.ErrBelegRetentionInvalid)
	}
	return requireTexts(map[string]string{"reason": suspension.Reason, "scope": suspension.Scope, "responsible": suspension.Responsible})
}

func requireTexts(fields map[string]string) error {
	return requireLength(fields, parentbilling.MaxInstructionTextLength)
}

func requireLength(fields map[string]string, limit int) error {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		value := strings.TrimSpace(fields[name])
		if value == "" || len([]rune(value)) > limit {
			return fmt.Errorf("%w: %s is required and at most %d characters", parentbilling.ErrBelegRetentionInvalid, name, limit)
		}
	}
	return nil
}
