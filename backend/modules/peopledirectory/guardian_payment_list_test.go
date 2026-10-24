package peopledirectory_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Every child keeps a line; a missing payer or IBAN is named, and the
// subtitle counts the gaps.
func TestGuardianPaymentListNamesEveryGap(t *testing.T) {
	t.Parallel()

	payer := int64(12)
	list := peopledirectory.GuardianPaymentListOf([]peopledirectory.GuardianPaymentRow{
		{StudentName: "Ada", SchoolClass: "3a", GuardianProfileID: &payer, AccountHolder: "Eva", IBAN: "DE02120300000000202051"},
		{StudentName: "Ben", SchoolClass: "3a", GuardianProfileID: &payer, AccountHolder: "Eva"},
		{StudentName: "Cem", SchoolClass: "4b"},
	})

	if list.Title != "Bankverbindungen" || list.Subtitle != "3 Kinder, davon 1 mit Bankverbindung" {
		t.Errorf("title %q, subtitle %q", list.Title, list.Subtitle)
	}
	if list.Footer != "Vertraulich. Enthält Bankverbindungen. Bitte nicht per E-Mail weitergeben." {
		t.Errorf("footer %q", list.Footer)
	}
	want := []peopledirectory.GuardianPaymentLine{
		{Student: "Ada", Class: "3a", AccountHolder: "Eva", IBAN: "DE02120300000000202051"},
		{Student: "Ben", Class: "3a", AccountHolder: "Eva", IBAN: "Fehlt"},
		{Student: "Cem", Class: "4b", AccountHolder: "Nicht zugeordnet", IBAN: "Fehlt"},
	}
	for i, line := range want {
		if list.Lines[i] != line {
			t.Errorf("line %d = %+v, want %+v", i, list.Lines[i], line)
		}
	}
	complete := peopledirectory.GuardianPaymentListOf([]peopledirectory.GuardianPaymentRow{{GuardianProfileID: &payer, IBANMasked: "DE02 **** 2051"}})
	if complete.Subtitle != "1 Kinder, alle mit Bankverbindung" {
		t.Errorf("complete subtitle %q", complete.Subtitle)
	}
	if list.Headings != (peopledirectory.GuardianPaymentLine{Student: "Kind", Class: "Klasse", AccountHolder: "Kontoinhaber", IBAN: "IBAN"}) {
		t.Errorf("headings %+v", list.Headings)
	}
}

// A payment row cleared of both fields reads as "no bank details", not as an
// empty-but-present record.
func TestGuardianPaymentHasData(t *testing.T) {
	t.Parallel()

	empty, iban, holder := "", "DE89", "A"
	cases := []struct {
		row  peopledirectory.GuardianPayment
		want bool
	}{
		{peopledirectory.GuardianPayment{GuardianProfileID: 7}, false},
		{peopledirectory.GuardianPayment{GuardianProfileID: 7, IBAN: &empty}, false},
		{peopledirectory.GuardianPayment{GuardianProfileID: 7, IBAN: &iban}, true},
		{peopledirectory.GuardianPayment{GuardianProfileID: 7, AccountHolder: &holder}, true},
	}
	for i, tc := range cases {
		if got := tc.row.HasData(); got != tc.want {
			t.Errorf("case %d: HasData() = %v, want %v", i, got, tc.want)
		}
	}
}
