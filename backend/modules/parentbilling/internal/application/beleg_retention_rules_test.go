package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

func date(y, m, d int) calendar.Date { return calendar.NewDate(y, time.Month(m), d) }

func datePtr(y, m, d int) *calendar.Date { v := date(y, m, d); return &v }

func eightYears(belegType string) *parentbilling.RetentionInstruction {
	return &parentbilling.RetentionInstruction{ID: 7, BelegType: belegType, TriggerEvent: parentbilling.TriggerIssued, Period: 8}
}

// An invoice issued in 2026 may go on 1 January 2035 at the earliest: eight
// years after the end of its issue year, whether issued on 1 January or on
// 31 December (Löschkonzept section 2).
func TestBookingBelegEndsEightYearsAfterTheEndOfItsIssueYear(t *testing.T) {
	t.Parallel()
	for _, issued := range []calendar.Date{date(2026, 1, 1), date(2026, 12, 31)} {
		decision := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareInvoice, Issued: &issued},
			eightYears(parentbilling.BelegCareInvoice), nil, date(2026, 12, 31))
		assert.Equal(t, parentbilling.RetentionScheduled, decision.State, issued.String())
		assert.Equal(t, datePtr(2035, 1, 1), decision.DeletableOn, issued.String())
		assert.Equal(t, &issued, decision.TriggerDate)
		assert.Equal(t, eightYears("").ID, decision.InstructionID)
		assert.False(t, decision.Due(date(2034, 12, 31)), "the last day of 2034 still keeps it")
		assert.True(t, decision.Due(date(2035, 1, 1)), "the first day of 2035 deletes it")
	}
	issued := date(2027, 1, 1)
	decision := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegHolidayDocument, Issued: &issued},
		eightYears(parentbilling.BelegHolidayDocument), nil, date(2027, 1, 1))
	assert.Equal(t, datePtr(2036, 1, 1), decision.DeletableOn, "a Beleg of the next year starts a year later")
}

// Without an instruction nothing is deleted: the Beleg goes on the
// clarification list for 30 days, and a booking Beleg shows the proposed
// eight years; a contract or notice proposes nothing.
func TestBelegWithoutInstructionWaitsOnTheClarificationList(t *testing.T) {
	t.Parallel()
	issued := date(2026, 3, 2)
	decision := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegHolidayDocument, Issued: &issued}, nil, nil, date(2040, 1, 1))
	assert.Equal(t, parentbilling.RetentionClarification, decision.State)
	assert.Equal(t, parentbilling.ClarifyNoInstruction, decision.Reason)
	assert.Nil(t, decision.DeletableOn)
	assert.Equal(t, datePtr(2035, 1, 1), decision.ProposedOn)
	assert.False(t, decision.Due(date(2040, 1, 1)), "no instruction, no deletion, however late")

	notice := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareNotice, Issued: &issued}, nil, nil, date(2026, 4, 1))
	assert.Equal(t, parentbilling.RetentionClarification, notice.State)
	assert.Nil(t, notice.ProposedOn, "a notice has no default period")
}

// A contract follows the event its instruction names: six years after the
// end of the year it ended, and while in force it waits.
func TestContractFollowsTheInstructedEvent(t *testing.T) {
	t.Parallel()
	ended := &parentbilling.RetentionInstruction{ID: 3, TriggerEvent: parentbilling.TriggerContractEnded, Period: 6}
	running := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, ConcludedOn: datePtr(2026, 8, 1)}, ended, nil, date(2027, 1, 1))
	assert.Equal(t, parentbilling.RetentionRunning, running.State)
	assert.Nil(t, running.DeletableOn)

	future := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, Ended: true, EndedOn: datePtr(2027, 7, 31)}, ended, nil, date(2027, 7, 30))
	assert.Equal(t, parentbilling.RetentionRunning, future.State, "a notice to a later end keeps it in force")

	over := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, Ended: true, EndedOn: datePtr(2027, 7, 31)}, ended, nil, date(2027, 7, 31))
	assert.Equal(t, parentbilling.RetentionScheduled, over.State)
	assert.Equal(t, datePtr(2034, 1, 1), over.DeletableOn)

	withdrawn := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, Ended: true}, ended, nil, date(2027, 1, 1))
	assert.Equal(t, parentbilling.RetentionClarification, withdrawn.State, "an end without a day gets no invented date")
	assert.Equal(t, parentbilling.ClarifyNoTriggerDate, withdrawn.Reason)

	concluded := &parentbilling.RetentionInstruction{ID: 4, TriggerEvent: parentbilling.TriggerContractConcluded, Period: 8}
	inForce := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, ConcludedOn: datePtr(2026, 8, 1)}, concluded, nil, date(2040, 1, 1))
	assert.Equal(t, parentbilling.RetentionRunning, inForce.State, "a contract in force stays, even counted from its conclusion")
	afterEnd := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, ConcludedOn: datePtr(2026, 8, 1), Ended: true, EndedOn: datePtr(2030, 7, 31)}, concluded, nil, date(2031, 1, 1))
	assert.Equal(t, datePtr(2035, 1, 1), afterEnd.DeletableOn, "counted from the conclusion once ended")
	missing := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegCareContract, Ended: true, EndedOn: datePtr(2027, 7, 31)}, concluded, nil, date(2027, 8, 1))
	assert.Equal(t, parentbilling.RetentionClarification, missing.State)
	assert.Equal(t, parentbilling.ClarifyNoTriggerDate, missing.Reason)
}

// A SEPA mandate is kept while valid; after its expiry at least 14 months
// from the last collected debit. Without a collection it needs clarifying.
func TestMandateIsKeptFourteenMonthsAfterItsLastCollection(t *testing.T) {
	t.Parallel()
	months := &parentbilling.RetentionInstruction{ID: 9, TriggerEvent: parentbilling.TriggerLastCollection, Period: 14}
	valid := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegSEPAMandate, SignedOn: datePtr(2026, 1, 10), LastCollection: datePtr(2026, 11, 2)}, months, nil, date(2027, 1, 1))
	assert.Equal(t, parentbilling.RetentionRunning, valid.State)

	revoked := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegSEPAMandate, SignedOn: datePtr(2026, 1, 10), LastCollection: datePtr(2026, 11, 2), RevokedOn: datePtr(2026, 12, 15)}, months, nil, date(2026, 12, 15))
	assert.Equal(t, parentbilling.RetentionScheduled, revoked.State)
	assert.Equal(t, datePtr(2028, 1, 2), revoked.DeletableOn)
	assert.Equal(t, datePtr(2026, 11, 2), revoked.TriggerDate)

	unused := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegSEPAMandate, SignedOn: datePtr(2026, 1, 10), LastCollection: datePtr(2026, 11, 2)}, months, nil, date(2029, 11, 2))
	assert.Equal(t, parentbilling.RetentionScheduled, unused.State, "36 months without a collection end the mandate")
	assert.Equal(t, datePtr(2028, 1, 2), unused.DeletableOn)

	never := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegSEPAMandate, SignedOn: datePtr(2026, 1, 10), RevokedOn: datePtr(2026, 6, 1)}, months, nil, date(2026, 6, 1))
	assert.Equal(t, parentbilling.RetentionClarification, never.State)
	assert.Equal(t, parentbilling.ClarifyNoCollection, never.Reason)
	assert.Nil(t, never.DeletableOn, "no date from an invented collection")

	fresh := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegSEPAMandate, SignedOn: datePtr(2026, 1, 10)}, months, nil, date(2026, 6, 1))
	assert.Equal(t, parentbilling.RetentionRunning, fresh.State, "a valid mandate not yet used waits")
}

// A suspension keeps the computed end but holds the deletion.
func TestSuspensionHoldsTheDeletion(t *testing.T) {
	t.Parallel()
	issued := date(2026, 5, 1)
	hold := &parentbilling.RetentionSuspension{ID: 11}
	decision := DecideBelegRetention(parentbilling.BelegFact{BelegType: parentbilling.BelegSEPADebit, Issued: &issued}, eightYears(parentbilling.BelegSEPADebit), hold, date(2036, 1, 1))
	assert.Equal(t, parentbilling.RetentionSuspended, decision.State)
	assert.Equal(t, hold.ID, decision.SuspensionID)
	assert.Equal(t, datePtr(2035, 1, 1), decision.DeletableOn)
	assert.False(t, decision.Due(date(2036, 1, 1)))
}

func TestInstructionValidationFollowsTheType(t *testing.T) {
	t.Parallel()
	base := parentbilling.NewRetentionInstruction{LegalBasis: "§ 147 AO", InstructedBy: "Kämmerei", InstructedOn: date(2026, 11, 9), InstructionReference: "Nachtrag 2026-11"}
	valid := []parentbilling.NewRetentionInstruction{
		withBasis(withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 8), parentbilling.BasisBookingVoucher),
		withBasis(withType(base, parentbilling.BelegCareContract, parentbilling.TriggerContractEnded, 6), parentbilling.BasisBusinessLetter),
		withBasis(withType(base, parentbilling.BelegSEPAMandate, parentbilling.TriggerLastCollection, 14), parentbilling.BasisCollectionAgreement),
	}
	for _, instruction := range valid {
		assert.NoError(t, ValidateRetentionInstruction(instruction), instruction.BelegType)
	}
	invalid := []parentbilling.NewRetentionInstruction{
		withType(base, "receipt", parentbilling.TriggerIssued, 8),
		withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerContractEnded, 8),
		withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 0),
		withType(base, parentbilling.BelegSEPAMandate, parentbilling.TriggerLastCollection, 13),
		withType(parentbilling.NewRetentionInstruction{InstructedOn: date(2026, 11, 9)}, parentbilling.BelegCareNotice, parentbilling.TriggerIssued, 8),
	}
	for _, instruction := range invalid {
		assert.ErrorIs(t, ValidateRetentionInstruction(instruction), parentbilling.ErrBelegRetentionInvalid, "%+v", instruction)
	}
}

func withBasis(instruction parentbilling.NewRetentionInstruction, basis string) parentbilling.NewRetentionInstruction {
	instruction.Basis = basis
	return instruction
}

func withType(base parentbilling.NewRetentionInstruction, belegType, trigger string, period int) parentbilling.NewRetentionInstruction {
	base.BelegType, base.TriggerEvent, base.Period = belegType, trigger, period
	return base
}

// An invoice, credit or cancellation note and a submitted debit are booking
// vouchers: a Weisung citing § 147 AO, § 257 HGB or § 14b UStG cannot keep
// them shorter than eight years after the end of their issue year
// (Löschkonzept section 2).
func TestBookingInstructionCitingTheStatutesKeepsEightYears(t *testing.T) {
	t.Parallel()
	for _, belegType := range []string{parentbilling.BelegHolidayDocument, parentbilling.BelegCareInvoice, parentbilling.BelegSEPADebit} {
		for _, basis := range []string{"§ 147 AO", "§ 257 HGB", "§ 14b UStG"} {
			short := parentbilling.NewRetentionInstruction{BelegType: belegType, TriggerEvent: parentbilling.TriggerIssued, Period: 1,
				LegalBasis: basis, InstructedBy: "Kämmerei", InstructedOn: date(2026, 11, 9), InstructionReference: "Nachtrag 2026-11"}
			for _, kind := range []string{"", parentbilling.BasisBookingVoucher, parentbilling.BasisNotApplicable} {
				short.Basis, short.BasisJustification = kind, ""
				if kind == parentbilling.BasisNotApplicable {
					short.BasisJustification = "nur ein Jahr gewünscht"
				}
				assert.ErrorIs(t, ValidateRetentionInstruction(short), parentbilling.ErrBelegRetentionInvalid, "%s with %s as %q", belegType, basis, kind)
			}
		}
	}
}

// statuteSpellings cite § 147 AO, § 257 HGB or § 14b UStG without the
// spaces and word boundaries the earlier pattern needed; the migration test
// TestInstructionTableRefusesEverySpellingOfTheStatutes holds the database
// to the same list.
var statuteSpellings = []string{
	"§147AO", "§ 147 Abs. 1 AO", "147ao", "HGB§257", "UStG14b", "Abgabenordnung", "ABGABENORDNUNG", "aBgAbEnOrDnUnG",
	"Handelsgesetzbuch", "Umsatzsteuergesetz", "§\u00a0147\u00a0AO", "§\u2009257\u2009HGB", "§\u202f14b\u202fUStG", "A.O.", "A-O",
	"§ 14 b UStG", "§ 257 Abs. 4 Nr. 1 HGB", "Paragraph 147 der Abgaben-Ordnung", "\uff21\uff2f", "A\u200bO", "nach §147",
}

// A Weisung whose statutes do not apply cites none of them, however it is
// spelled (#4720, third review): a citation hidden by missing spaces,
// unicode spaces or case kept a 2026 invoice for one year.
func TestNotApplicableRefusesEverySpellingOfTheStatutes(t *testing.T) {
	t.Parallel()
	short := parentbilling.NewRetentionInstruction{BelegType: parentbilling.BelegCareInvoice, TriggerEvent: parentbilling.TriggerIssued,
		Period: 1, Basis: parentbilling.BasisNotApplicable, BasisJustification: "Träger ist nicht buchführungspflichtig",
		InstructedBy: "Kämmerei", InstructedOn: date(2026, 11, 9), InstructionReference: "Nachtrag 2026-11"}
	for _, belegType := range []string{parentbilling.BelegHolidayDocument, parentbilling.BelegCareInvoice, parentbilling.BelegSEPADebit,
		parentbilling.BelegCareNotice, parentbilling.BelegCareContract} {
		short.BelegType, short.TriggerEvent = belegType, parentbilling.TriggerIssued
		if belegType == parentbilling.BelegCareContract {
			short.TriggerEvent = parentbilling.TriggerContractEnded
		}
		for _, spelling := range statuteSpellings {
			short.LegalBasis = spelling
			assert.ErrorIs(t, ValidateRetentionInstruction(short), parentbilling.ErrBelegRetentionInvalid, "%s: %q", belegType, spelling)
		}
		for _, basis := range []string{"Satzung des Trägers § 12", "Beschluss des Vorstands vom 01.02.2026", "Kommunalhaushaltsverordnung NRW § 58"} {
			short.LegalBasis = basis
			assert.NoError(t, ValidateRetentionInstruction(short), "%s: %q", belegType, basis)
		}
	}
}

// The basis decides the least period: a booking voucher eight years, a
// business letter six; only the written statement that the statutes do not
// apply allows less, and it cites none of them. A business letter is a
// classification of contracts and notices, not of an invoice.
func TestInstructionBasisSetsTheLeastPeriod(t *testing.T) {
	t.Parallel()
	base := parentbilling.NewRetentionInstruction{LegalBasis: "Satzung des Trägers", InstructedBy: "Frau Beispiel, Kämmerei",
		InstructedOn: date(2026, 11, 9), InstructionReference: "Nachtrag 2026-11"}
	notApplicable := func(instruction parentbilling.NewRetentionInstruction, justification string) parentbilling.NewRetentionInstruction {
		instruction.Basis, instruction.BasisJustification = parentbilling.BasisNotApplicable, justification
		return instruction
	}
	valid := []parentbilling.NewRetentionInstruction{
		withBasis(withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 8), parentbilling.BasisBookingVoucher),
		withBasis(withType(base, parentbilling.BelegCareNotice, parentbilling.TriggerIssued, 8), parentbilling.BasisBookingVoucher),
		withBasis(withType(base, parentbilling.BelegCareNotice, parentbilling.TriggerIssued, 6), parentbilling.BasisBusinessLetter),
		notApplicable(withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 1), "Träger ist nicht buchführungspflichtig, Bescheid des Finanzamts vom 01.02.2026"),
		notApplicable(withType(base, parentbilling.BelegCareContract, parentbilling.TriggerContractEnded, 3), "Verträge sind weder Buchungsbeleg noch Geschäftsbrief des Trägers"),
	}
	for _, instruction := range valid {
		assert.NoError(t, ValidateRetentionInstruction(instruction), "%+v", instruction)
	}
	cited := withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 1)
	cited.LegalBasis = "§ 147 AO"
	invalid := map[string]parentbilling.NewRetentionInstruction{
		"booking voucher below eight years":   withBasis(withType(base, parentbilling.BelegSEPADebit, parentbilling.TriggerIssued, 7), parentbilling.BasisBookingVoucher),
		"business letter below six years":     withBasis(withType(base, parentbilling.BelegCareContract, parentbilling.TriggerContractEnded, 5), parentbilling.BasisBusinessLetter),
		"contract as booking voucher below 8": withBasis(withType(base, parentbilling.BelegCareContract, parentbilling.TriggerContractEnded, 6), parentbilling.BasisBookingVoucher),
		"invoice as business letter":          withBasis(withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 6), parentbilling.BasisBusinessLetter),
		"no basis":                            withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 8),
		"mandate without the agreement":       withBasis(withType(base, parentbilling.BelegSEPAMandate, parentbilling.TriggerLastCollection, 14), parentbilling.BasisNotApplicable),
		"not applicable without a reason":     notApplicable(withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 1), "  "),
		"not applicable citing § 147 AO":      notApplicable(cited, "Träger ist nicht buchführungspflichtig"),
		"justification on a statutory basis": func() parentbilling.NewRetentionInstruction {
			instruction := withBasis(withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 8), parentbilling.BasisBookingVoucher)
			instruction.BasisJustification = "überflüssig"
			return instruction
		}(),
	}
	for name, instruction := range invalid {
		assert.ErrorIs(t, ValidateRetentionInstruction(instruction), parentbilling.ErrBelegRetentionInvalid, name)
	}
}

// A review notes what was clarified, names the reason of an escalation and
// sets the next review after today, within a year.
func TestClarificationReviewNeedsANoteAndANextReview(t *testing.T) {
	t.Parallel()
	today := date(2026, 12, 10)
	base := parentbilling.NewBelegClarificationReview{BelegType: parentbilling.BelegCareContract, Reason: parentbilling.ClarifyNoInstruction,
		Note: "Träger prüft Einordnung der Verträge", NextReviewOn: date(2027, 1, 10)}
	assert.NoError(t, ValidateClarificationReview(base, today))
	escalated := base
	escalated.Escalated, escalated.EscalationReason = true, "Träger antwortet seit 30 Tagen nicht"
	assert.NoError(t, ValidateClarificationReview(escalated, today))

	for name, change := range map[string]func(*parentbilling.NewBelegClarificationReview){
		"no note":                      func(r *parentbilling.NewBelegClarificationReview) { r.Note = " " },
		"escalation without a reason":  func(r *parentbilling.NewBelegClarificationReview) { r.Escalated = true },
		"reason without an escalation": func(r *parentbilling.NewBelegClarificationReview) { r.EscalationReason = "dringend" },
		"next review today":            func(r *parentbilling.NewBelegClarificationReview) { r.NextReviewOn = today },
		"next review beyond a year":    func(r *parentbilling.NewBelegClarificationReview) { r.NextReviewOn = today.AddDays(366) },
		"unknown reason":               func(r *parentbilling.NewBelegClarificationReview) { r.Reason = "other" },
		"unknown type":                 func(r *parentbilling.NewBelegClarificationReview) { r.BelegType = "receipt" },
	} {
		review := base
		change(&review)
		assert.ErrorIs(t, ValidateClarificationReview(review, today), parentbilling.ErrBelegRetentionInvalid, name)
	}
}

// An entry still open at its review goes to Florian with a reason before
// its review moves (Löschkonzept section 3, #4720 third review): a new date
// alone cannot clear an overdue entry, and a review before the deadline
// cannot push it out without escalation. Keeping or advancing it can.
func TestClarificationReviewEscalatesBeforeTheReviewMoves(t *testing.T) {
	t.Parallel()
	entryReviewOn := date(2026, 12, 9) // listed on 9 November, thirty days
	open := parentbilling.NewBelegClarificationReview{BelegType: parentbilling.BelegCareContract, Reason: parentbilling.ClarifyNoInstruction,
		Note: "Träger hat noch nicht geantwortet", NextReviewOn: date(2027, 1, 10)}
	escalated := open
	escalated.Escalated, escalated.EscalationReason = true, "Keine Antwort seit 30 Tagen"
	assert.NoError(t, ValidateClarificationEscalation(escalated, entryReviewOn))
	for name, next := range map[string]calendar.Date{"past the deadline": date(2027, 1, 10), "one day past it": date(2026, 12, 10)} {
		review := open
		review.NextReviewOn = next
		assert.ErrorIs(t, ValidateClarificationEscalation(review, entryReviewOn), parentbilling.ErrBelegRetentionInvalid, name)
	}
	for name, next := range map[string]calendar.Date{"keeps the deadline": entryReviewOn, "advances it": date(2026, 11, 20)} {
		review := open
		review.NextReviewOn = next
		assert.NoError(t, ValidateClarificationEscalation(review, entryReviewOn), name)
	}
}
