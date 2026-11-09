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
		withType(base, parentbilling.BelegCareInvoice, parentbilling.TriggerIssued, 8),
		withType(base, parentbilling.BelegCareContract, parentbilling.TriggerContractEnded, 6),
		withType(base, parentbilling.BelegSEPAMandate, parentbilling.TriggerLastCollection, 14),
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

func withType(base parentbilling.NewRetentionInstruction, belegType, trigger string, period int) parentbilling.NewRetentionInstruction {
	base.BelegType, base.TriggerEvent, base.Period = belegType, trigger, period
	return base
}
