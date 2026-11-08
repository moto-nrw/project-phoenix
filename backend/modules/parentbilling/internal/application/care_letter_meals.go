package application

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// careMealFacts prints one line per Mensa month of an invoice (#4686), as
// MK-001 writes it: „2 Mittagessen im Januar 2026 à 4,80 €“, or the flat
// amount of the month. sign -1 prints the amounts of a Storno.
func careMealFacts(positions []parentbilling.CareMealPosition, sign int64) []LetterFact {
	facts := make([]LetterFact, 0, len(positions))
	for _, position := range positions {
		facts = append(facts, LetterFact{Label: mealPositionLabel(position), Value: formatEuro(sign * position.AmountCents)})
	}
	return facts
}

// mealPositionLabel names a Mensa month as the letter and the review list
// print it.
func mealPositionLabel(position parentbilling.CareMealPosition) string {
	if position.Mode == parentbilling.MealModePerMeal {
		return fmt.Sprintf("%d Mittagessen im %s à %s", position.Quantity, careMonthName(position.MealMonth), formatEuro(position.UnitCents))
	}
	return fmt.Sprintf("Mensa-Pauschale %s", careMonthName(position.MealMonth))
}

// careBasisText says what an invoice bills by: the contract, and the meals
// the OGS released when it bills Mensa months.
func careBasisText(invoice parentbilling.CareInvoice) string {
	if len(invoice.MealPositions) == 0 {
		return "Abgerechnet wird nach Ihrem Vertrag, nicht nach Anwesenheit."
	}
	if len(invoice.Positions) == 0 {
		return "Abgerechnet wird das Mittagessen, wie die OGS es für den Monat bestätigt hat."
	}
	return "Die Betreuung rechnen wir nach Ihrem Vertrag ab. Das Mittagessen rechnen wir so ab, wie die OGS es für den Monat bestätigt hat."
}
