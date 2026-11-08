package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The monthly letter prints the Mensa months (#4686) as MK-001 does, after
// the modules and reductions, so the lines add up to the sum; the Storno
// takes them back with a minus.
func TestCareLettersPrintTheMensaMonthsOfTheInvoice(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.invoice.MealPositions = []parentbilling.CareMealPosition{
		{MealMonth: calendar.NewDate(2026, 12, 1), Mode: parentbilling.MealModePerMeal, Quantity: 2, UnitCents: 480, AmountCents: 960},
		{MealMonth: calendar.NewDate(2027, 1, 1), Mode: parentbilling.MealModeFlat, Quantity: 1, UnitCents: 5500, AmountCents: 5500},
	}
	content.invoice.TotalCents = 5401 + 960 + 5500
	text := careLetterText(careInvoiceLetter(content))
	assert.Contains(t, text, "2 Mittagessen im Dezember 2026 à 4,80 € = 9,60 €")
	assert.Contains(t, text, "Mensa-Pauschale Januar 2027 = 55,00 €")
	assert.Contains(t, text, "Summe = 118,61 €")
	assert.Contains(t, text, "Das Mittagessen rechnen wir so ab, wie die OGS es für den Monat bestätigt hat.")
	assert.NotContains(t, text, "nicht nach Anwesenheit", "the Mensa is billed by the meals")
	assert.Less(t, strings.Index(text, "Betreuung bis 16:30 Uhr"), strings.Index(text, "2 Mittagessen"), "modules first, then the Mensa")

	content.cancellation = &parentbilling.CareCancellation{Number: "MB-2027-0009", IssuedOn: calendar.NewDate(2027, 1, 10), Reason: "x", TotalCents: -11861}
	storno := careLetterText(careCancellationLetter(content))
	assert.Contains(t, storno, "2 Mittagessen im Dezember 2026 à 4,80 € = -9,60 €")
	assert.Contains(t, storno, "Storno gesamt = -118,61 €")
}

// A Mensa-only invoice says it bills the lunch, not a contract.
func TestCareLetterOfAMensaOnlyInvoice(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.invoice.Positions = nil
	content.invoice.MealPositions = []parentbilling.CareMealPosition{
		{MealMonth: calendar.NewDate(2026, 1, 1), Mode: parentbilling.MealModePerMeal, Quantity: 2, UnitCents: 480, AmountCents: 960},
	}
	content.invoice.TotalCents = 960
	text := careLetterText(careInvoiceLetter(content))
	assert.Contains(t, text, "Abgerechnet wird das Mittagessen, wie die OGS es für den Monat bestätigt hat.")
	assert.Contains(t, text, "Summe = 9,60 €")
}
