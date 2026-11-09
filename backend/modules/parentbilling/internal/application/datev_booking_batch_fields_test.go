package application

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The text fields of the DATEV booking batch (#4694, design in PR #6360):
// one rule per field (invariant 1), values of settings and header refused,
// never changed (2), only the Buchungstext replaced (3), and the writer and
// encoder refuse instead of replacing (4).

// Invariant 1: every rule of the table, with the border cases.
func TestDatevTextRules(t *testing.T) {
	t.Parallel()
	r := datevTextRules
	cases := []struct {
		name    string
		rule    datevTextRule
		allowed []string
		refused []string
	}{
		{"Kopf 1 Kennzeichen", r.kennzeichen, []string{"EXTF"}, []string{"", "extf", "DTVF "}},
		{"Kopf 4 Formatname", r.formatname, []string{"Buchungsstapel"}, []string{"", "Debitoren/Kreditoren"}},
		{"Kopf 8 Herkunft", r.herkunft, []string{"", "RE", "a_"}, []string{"REX", "R;", "Ä"}},
		{"Kopf 9 Exportiert von", r.exportedBy, []string{"", strings.Repeat("a", 25)}, []string{strings.Repeat("a", 26), "Max Mustermann", "²"}},
		{"Kopf 10 Importiert von", r.importedBy, []string{"", "Admin"}, []string{"Ad min", "Admin\t"}},
		{"Kopf 17 Bezeichnung", r.label,
			[]string{"", "Elternbeitraege 10/2026", "a_b.c/d e", strings.Repeat("A", 30)},
			[]string{strings.Repeat("A", 31), "Beitrag ²", "Beitrag ł", "Beiträge", "Beitrag 😀", "Beitrag 東京", "Beitrag\t10", "Beitrag;10", `Beitrag "10"`, "Beitrag-10", "Beitrag: 10", "Beitrag, 10"}},
		{"Kopf 18 Diktatkürzel", r.dictation, []string{"", "MM", "MMAB"}, []string{"M", "mm", "MMA", "ÄÖ", "MMABCD"}},
		{"Kopf 22 WKZ", r.currency, []string{"EUR", "CHF"}, []string{"", "eur", "EU", "EURO"}},
		{"Kopf 24/30, ungenutzte Spalten", r.empty, []string{""}, []string{" ", "x"}},
		{"Kopf 27 Sachkontenrahmen", r.chart, []string{"", "03", "04", "0304"}, []string{"3", "SKR04", "030", "04 "}},
		{"Kopf 31 Anwendungsinformation", r.applicationInfo, []string{"", "02/2024", strings.Repeat("x", 16)}, []string{strings.Repeat("x", 17), "a\nb", "😀"}},
		{"Buchung 2 Soll/Haben", r.side, []string{"S", "H"}, []string{"", "s", "X", "SH"}},
		{"Buchung 11 Belegfeld 1", r.documentNumber,
			[]string{"MB-2026/0001", "A_$&%*+-/", strings.Repeat("9", 36)},
			[]string{"", strings.Repeat("9", 37), "MB 1", "Gebühr1", "MB²", "MB;1", `MB"1`, "MB.1", "MB,1", "MB:1", "東1", "MB\t1"}},
		{"Buchung 14 Buchungstext", r.text,
			[]string{"", "Müller ² € ;", `Gutschrift "Geschwister"`, strings.Repeat("ä", 60)},
			[]string{strings.Repeat("a", 61), "Beitrag 😀", "Beitrag 東京", "Beitrag ł", "a\tb", "a\nb", "a\u0081b"}},
		{"Buchung 37 KOST1", r.costCenter1, []string{"", "Modul 2_A", strings.Repeat("K", 36)}, []string{strings.Repeat("K", 37), "Modul-2", "A;B", "Ä", "²"}},
		{"Buchung 38 KOST2", r.costCenter2, []string{"", "Ispringen"}, []string{"Ispringen/Nord", `"A"`}},
	}
	for _, tc := range cases {
		for _, value := range tc.allowed {
			assert.True(t, tc.rule.allows(value), "%s admits %q", tc.name, value)
		}
		for _, value := range tc.refused {
			assert.False(t, tc.rule.allows(value), "%s refuses %q", tc.name, value)
		}
	}
}

// Invariant 2: values of the header and the booking keys come back as a
// problem with their field, never changed into a valid value.
func TestDatevHeaderAndKeysAreNeverChanged(t *testing.T) {
	t.Parallel()
	for _, label := range []string{"Beitrag ²", "Beitrag ł", strings.Repeat("A", 31), "Beitrag 東京"} {
		header := datevTestHeader()
		header.Label = label
		_, err := FormatDatevBookingBatch(header, nil)
		var formatErr *DatevFormatError
		require.True(t, errors.As(err, &formatErr), "label %q", label)
		assert.Equal(t, []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}, formatErr.Problems, "label %q", label)
	}
	header := datevTestHeader()
	header.ChartOfAccounts = "SKR04"
	_, err := FormatDatevBookingBatch(header, nil)
	var formatErr *DatevFormatError
	require.True(t, errors.As(err, &formatErr))
	assert.Equal(t, []DatevProblem{{Field: DatevFieldChartOfAccounts, Reason: DatevInvalid}}, formatErr.Problems)
}

// Invariant 3: only the Buchungstext is replaced, onto its own rule.
func TestDatevPersonText(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Wałęsa 😀 東京\tMüller":             "Walesa Müller",
		"Ődön\nĐorđe":                     "Odön Dorde",
		"Weiß ² € ; \"Kind\"":             "Weiß ² € ; \"Kind\"",
		strings.Repeat("ä", 70):           strings.Repeat("ä", 60),
		"  viele   Leerzeichen  ":         "viele Leerzeichen",
		"東京":                              "",
		"Beitrag\u0081Oktober":            "Beitrag Oktober",
		strings.Repeat("a", 59) + " 東 bc": strings.Repeat("a", 59) + " ",
	}
	for in, want := range cases {
		got := datevPersonText(in)
		assert.Equal(t, strings.TrimSpace(want), got, "%q", in)
		assert.True(t, datevTextRules.text.allows(got), "%q → %q stays inside the rule", in, got)
	}
	booking := datevTestBookings()[0]
	booking.Text = "Beitrag Wałęsa 😀"
	file, err := FormatDatevBookingBatch(datevTestHeader(), []DatevBooking{booking})
	require.NoError(t, err, "a name in another script does not block the export")
	assert.Contains(t, string(file.Content), `"Beitrag Walesa"`)
}

// Invariant 4: the line writer checks every text field when it writes it,
// independent of the checks before.
func TestDatevLineWriterRefusesTextOutsideItsRule(t *testing.T) {
	t.Parallel()
	w := &datevLineWriter{line: 3}
	assert.Equal(t, `"a""b"`, w.text(datevTextRules.text, `a"b`))
	assert.Empty(t, w.problems)
	w.text(datevTextRules.text, "Beitrag 東京")
	w.text(datevTextRules.label, "Beitrag ²")
	w.text(datevTextRules.empty, "x")
	assert.Equal(t, []DatevProblem{
		{Line: 3, Field: DatevFieldText, Reason: DatevInvalid},
		{Line: 3, Field: DatevFieldLabel, Reason: DatevInvalid},
		{Line: 3, Field: datevFieldUnused, Reason: DatevInvalid},
	}, w.problems)

	header := datevTestHeader()
	booking := datevTestBookings()[0]
	booking.Side = "s"
	w = &datevLineWriter{line: 1}
	datevBookingLine(w, booking, false)
	assert.Equal(t, []DatevProblem{{Line: 1, Field: DatevFieldSide, Reason: DatevInvalid}}, w.problems,
		"the booking line checks its fields itself")
	w = &datevLineWriter{}
	header.Label = "Beitrag ł"
	header.CreatedAt = time.Date(2026, time.October, 30, 12, 0, 0, 0, calendar.Berlin)
	datevHeaderLine(w, header)
	assert.Equal(t, []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}, w.problems,
		"the header line checks its fields itself")
}
