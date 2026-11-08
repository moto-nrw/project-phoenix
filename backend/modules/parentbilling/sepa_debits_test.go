package parentbilling_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A mandate holds 36 months after its last collection, or after its
// signature without one (#4697, SEPA Core Rulebook 4.5).
func TestMandateExpiresThirtySixMonthsAfterTheLastCollection(t *testing.T) {
	t.Parallel()
	signed := calendar.NewDate(2023, 9, 15)
	assert.Equal(t, calendar.NewDate(2026, 9, 15), parentbilling.MandateExpiresOn(signed, nil), "never collected: from the signature")
	collected := calendar.NewDate(2025, 1, 31)
	assert.Equal(t, calendar.NewDate(2028, 1, 31), parentbilling.MandateExpiresOn(signed, &collected))
	leap := calendar.NewDate(2024, 2, 29)
	assert.Equal(t, calendar.NewDate(2027, 2, 28), parentbilling.MandateExpiresOn(signed, &leap), "the month's last day when it is shorter")
	earlier := calendar.NewDate(2023, 1, 1)
	assert.Equal(t, calendar.NewDate(2026, 9, 15), parentbilling.MandateExpiresOn(signed, &earlier), "the later of signature and collection")

	mandate := parentbilling.SEPAMandate{SignedOn: signed}
	assert.False(t, mandate.Expired(calendar.NewDate(2026, 9, 14)))
	assert.True(t, mandate.Expired(calendar.NewDate(2026, 9, 15)))
}

// An expired mandate is no mandate for a debit: the invoice falls back like
// one without a mandate and names why.
func TestResolveDebitRefusesAnExpiredMandate(t *testing.T) {
	t.Parallel()
	expired := *sjrMandate
	expired.SignedOn = calendar.NewDate(2023, 1, 2)
	got := debitOf(sjrCreditor, &expired, sjrIssued)
	assert.Equal(t, parentbilling.PaymentTransfer, got.Method)
	assert.Equal(t, parentbilling.DebitGapExpired, got.DebitGap)
	assert.Nil(t, got.Mandate)

	collected := calendar.NewDate(2026, 6, 1)
	expired.LastCollectedOn = &collected
	got = debitOf(sjrCreditor, &expired, sjrIssued)
	assert.Equal(t, parentbilling.PaymentDirectDebit, got.Method, "a recent collection keeps the mandate")
	assert.NotNil(t, got.Mandate)
}

// A mandate already used elsewhere (taken over with sequence recurring,
// #6198) holds 36 months from the day moto took it over until moto
// collects with it; a mandate never used runs from its signature.
func TestTakenOverMandateRunsFromTheTakeOver(t *testing.T) {
	t.Parallel()
	takenOver := calendar.NewDate(2026, 11, 8)
	mandate := parentbilling.SEPAMandate{SignedOn: calendar.NewDate(2019, 3, 1), Sequence: parentbilling.SequenceRecurring,
		CreatedAt: takenOver.BerlinMidnight().Add(10 * time.Hour)}
	assert.Equal(t, calendar.NewDate(2029, 11, 8), mandate.ExpiryDay())
	assert.False(t, mandate.Expired(calendar.NewDate(2027, 1, 1)), "a used mandate taken over is no expired mandate")
	mandate.Sequence = parentbilling.SequenceFirst
	assert.True(t, mandate.Expired(calendar.NewDate(2027, 1, 1)), "never used: 36 months after the signature")
	collected := calendar.NewDate(2027, 2, 1)
	assert.Equal(t, &collected, parentbilling.MandateUsedOn(parentbilling.SequenceRecurring, &collected, takenOver))
}
