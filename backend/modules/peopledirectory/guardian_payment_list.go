package peopledirectory

import "fmt"

// GuardianPaymentList is the Bankverbindungen list as an export renders it:
// one line per child, and every gap named, so the list reads as a to-do
// rather than as complete.
type GuardianPaymentList struct {
	Title    string
	Subtitle string
	// Footer is stamped on every page.
	Footer string
	// Headings are the column labels, in the order of the line fields.
	Headings GuardianPaymentLine
	Lines    []GuardianPaymentLine
}

// GuardianPaymentLine is one line of the list, or its column labels.
type GuardianPaymentLine struct {
	Student       string
	Class         string
	AccountHolder string
	IBAN          string
}

// GuardianPaymentListOf lays out the export of the payment rows. Its
// subtitle states how complete the list is: a bank list that silently omits
// its gaps reads as finished when it is not.
func GuardianPaymentListOf(rows []GuardianPaymentRow) GuardianPaymentList {
	return GuardianPaymentList{
		Title:    "Bankverbindungen",
		Subtitle: guardianPaymentListSubtitle(rows),
		Footer:   "Vertraulich. Enthält Bankverbindungen. Bitte nicht per E-Mail weitergeben.",
		Headings: GuardianPaymentLine{Student: "Kind", Class: "Klasse", AccountHolder: "Kontoinhaber", IBAN: "IBAN"},
		Lines:    guardianPaymentLines(rows),
	}
}

func guardianPaymentListSubtitle(rows []GuardianPaymentRow) string {
	withIBAN := 0
	for _, row := range rows {
		if row.HasIBAN() {
			withIBAN++
		}
	}
	if withIBAN == len(rows) {
		return fmt.Sprintf("%d Kinder, alle mit Bankverbindung", len(rows))
	}
	return fmt.Sprintf("%d Kinder, davon %d mit Bankverbindung", len(rows), withIBAN)
}

// guardianPaymentLines keeps the line of a child without a payer or without
// an IBAN and says what is missing.
func guardianPaymentLines(rows []GuardianPaymentRow) []GuardianPaymentLine {
	lines := make([]GuardianPaymentLine, 0, len(rows))
	for _, row := range rows {
		holder, iban := row.AccountHolder, row.IBAN
		switch {
		case row.GuardianProfileID == nil:
			holder, iban = "Nicht zugeordnet", "Fehlt"
		case iban == "":
			iban = "Fehlt"
		}
		lines = append(lines, GuardianPaymentLine{Student: row.StudentName, Class: row.SchoolClass, AccountHolder: holder, IBAN: iban})
	}
	return lines
}
