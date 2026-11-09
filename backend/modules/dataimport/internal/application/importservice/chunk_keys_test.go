package importpkg

import "testing"

// Inputs the owners' LOWER(NORMALIZE(BTRIM(x), NFC)) treats as equal share a
// key; a coarser key only splits a segment more often.
func TestMatchKeyIsCoarserThanTheOwnerMatch(t *testing.T) {
	t.Parallel()
	for _, equal := range [][2]string{
		{"José", "José"},        // composed and decomposed
		{"GÓMEZ", "gómez"},       // case
		{"Kai", "Kai"},           // the Kelvin sign is NFC K
		{"Å", "Å"},               // the Ångström sign is NFC Å
		{"  Müller ", "Müller"}, // spaces and decomposition
		{"Anna\t", "Anna"},       // the import trims any whitespace before search and create
		{"\u00a0Anna", "anna"},   // no-break space
		{"ẞ", "ß"},               // capital sharp s
	} {
		if matchKey(equal[0]) != matchKey(equal[1]) {
			t.Errorf("matchKey(%q) = %q, matchKey(%q) = %q; want one key", equal[0], matchKey(equal[0]), equal[1], matchKey(equal[1]))
		}
	}
	for _, distinct := range [][2]string{{"Anna", "Anne"}, {"Ben", "Benn"}, {"Lena", "Lea"}} {
		if matchKey(distinct[0]) == matchKey(distinct[1]) {
			t.Errorf("matchKey(%q) = matchKey(%q); want different keys", distinct[0], distinct[1])
		}
	}
}
