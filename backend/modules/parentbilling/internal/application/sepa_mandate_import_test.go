package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The import reads the exports an office program writes in Germany: UTF-8
// with or without BOM or Windows-1252, separated by semicolon or comma.
func TestReadImportTableReadsEncodingsAndDelimiters(t *testing.T) {
	t.Parallel()
	const header = "Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;E-Mail\n"
	const row = "SJR-1;01.02.2024;DE89 3704 0044 0532 0130 00;Jürgen Müller;familie@example.org\n"
	windows, err := charmap.Windows1252.NewEncoder().String(header + row)
	require.NoError(t, err)
	comma := "\"Mandatsreferenz\",\"Mandatsdatum\",IBAN,Kontoinhaber,E-Mail\n\"SJR-1\",01.02.2024,DE89370400440532013000,\"Müller, Jürgen\",familie@example.org\n"
	for name, test := range map[string]struct {
		content           string
		encoding, delim   string
		holder, delimiter string
	}{
		"utf-8 semicolon":        {content: header + row, encoding: encodingUTF8, delim: ";", holder: "Jürgen Müller"},
		"utf-8 with bom":         {content: "\xEF\xBB\xBF" + header + row, encoding: encodingUTF8, delim: ";", holder: "Jürgen Müller"},
		"windows-1252":           {content: windows, encoding: encodingWindows1252, delim: ";", holder: "Jürgen Müller"},
		"comma with quotes":      {content: comma, encoding: encodingUTF8, delim: ",", holder: "Müller, Jürgen"},
		"semicolon chosen":       {content: header + row, encoding: encodingUTF8, delim: ";", holder: "Jürgen Müller", delimiter: ";"},
		"crlf and an empty line": {content: "Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;E-Mail\r\n\r\n" + row, encoding: encodingUTF8, delim: ";", holder: "Jürgen Müller"},
	} {
		table, err := readImportTable([]byte(test.content), test.delimiter)
		require.NoError(t, err, name)
		assert.Equal(t, test.encoding, table.encoding, name)
		assert.Equal(t, test.delim, table.delimiter, name)
		require.Len(t, table.rows, 1, name)
		assert.Equal(t, test.holder, table.rows[0][3], name)
		assert.Equal(t, "Mandatsreferenz", table.headers[0], name)
	}
}

func TestReadImportTableRefusesWhatIsNoCSV(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"binary":        "PK\x03\x04\x00\x00binary",
		"header only":   "Mandatsreferenz;IBAN\n",
		"one column":    "Mandatsreferenz\nSJR-1\n",
		"empty":         "  \n",
		"nul in a cell": "a;b\nc;\x00\n",
		// Without a header row the first mandate would come back as the
		// headers, its IBAN unmasked.
		"repeated header":      "Name;IBAN;Name\nEva;DE89370400440532013000;Mia\n",
		"IBAN inside a header": "Mandatsreferenz;Konto DE89370400440532013000\nSJR-1;x\n",
		"no header row":        "SJR-1;01.02.2024;DE89 3704 0044 0532 0130 00;Eva\nSJR-2;01.02.2024;DE02120300000000202051;Jan\n",
	} {
		_, err := readImportTable([]byte(content), "")
		require.ErrorIs(t, err, parentbilling.ErrImportFileInvalid, name)
	}
	_, err := readImportTable(make([]byte, parentbilling.MaxImportFileBytes+1), "")
	require.ErrorIs(t, err, parentbilling.ErrImportFileTooLarge)
}

// ogsConnectMasterData is a file in the layout of the master data export of
// OGS Connect (#7009), with invented data: a row per child, NUL where a CSV
// has quotes, padded and empty headers, dates with a time of day and rows
// of children whose family pays without a mandate.
const ogsConnectMasterData = "\x00Nachname Kind \x00;\x00 Vorname Kind\x00;\x00\x00;\x00Debitor-Nr.\x00;\x00Kontoinhaber\x00;" +
	"\x00IBAN\x00;\x00Mandatsreferenz\x00;\x00Unterschriftsdatum\x00;\x00Mandat ist gültig ab\x00;\x00Lastschriftart\x00;\x00\x00\r\n" +
	"\x00Beispiel\x00;\x00Lia\x00;\x00\x00;\x0070001\x00;\x00Eva \"Evi\" Beispiel\x00;\x00DE89 3704 0044 0532 0130 00\x00;" +
	"\x00SJR-0001\x00;\x0001.02.2024 14:05\x00;\x0001.03.2024 00:00\x00;\x00CORE/RCUR\x00;\x00\x00\r\n" +
	"\x00Muster\x00;\x00Ben\x00;\x00\x00;\x0070002\x00;\x00\x00;\x00\x00;\x00\x00;\x00\x00;\x00\x00;\x00\x00;\x00\x00\r\n"

// The master data export of OGS Connect reads without any change to the
// file: NUL as quotes, the headers trimmed, the two unnamed columns kept
// apart from the named ones (#7009).
func TestReadImportTableReadsTheOGSConnectMasterData(t *testing.T) {
	t.Parallel()
	table, err := readImportTable([]byte(ogsConnectMasterData), "")
	require.NoError(t, err)
	assert.Equal(t, ";", table.delimiter)
	assert.Equal(t, []string{"Nachname Kind", "Vorname Kind", "", "Debitor-Nr.", "Kontoinhaber", "IBAN", "Mandatsreferenz",
		"Unterschriftsdatum", "Mandat ist gültig ab", "Lastschriftart", ""}, table.headers)
	require.Len(t, table.rows, 2)
	assert.Equal(t, "Eva \"Evi\" Beispiel", table.rows[0][4], "a quote inside a NUL field stays a quote")
	assert.Equal(t, "01.02.2024 14:05", table.rows[0][7])
	assert.Equal(t, []int{2, 3}, table.lines)

	mapping := suggestImportMapping(table.headers)
	assert.Equal(t, "Unterschriftsdatum", mapping.Columns.SignedOn)
	assert.Equal(t, "Debitor-Nr.", mapping.Columns.DebtorNumber)
	assert.Equal(t, "Lastschriftart", mapping.Columns.Sequence)

	columns := importColumnIndex{fields: map[string]int{parentbilling.ImportFieldIBAN: 5, parentbilling.ImportFieldReference: 6}}
	kept, without := dropRowsWithoutMandate(table, columns)
	assert.Equal(t, 1, without, "the child whose family pays without a mandate is no mistake")
	require.Len(t, kept.rows, 1)
	assert.Equal(t, []int{2}, kept.lines)
	assert.Len(t, table.rows, 2, "the file itself stays as read")
}

// A NUL counts as a quote only where a quote can stand; any other NUL
// keeps the file refused as no text.
func TestReadImportTableRefusesANULOutsideTheRoleOfAQuote(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"inside a cell":                       "Mandatsreferenz;IBAN\nSJR\x001;DE89370400440532013000\n",
		"never closed":                        "\x00Mandatsreferenz\x00;\x00IBAN\nSJR-1;x\n",
		"after the start":                     "Mandatsreferenz;IBAN\nSJR-1;D\x00E\x00\n",
		"inside a field of another delimiter": "Mandatsreferenz;Notiz\nSJR-1;a,\x00b\x00,c\n",
		"inside a quoted field":               "Mandatsreferenz;Notiz\nSJR-1;\"a\x00\x00b\"\n",
	} {
		_, err := readImportTable([]byte(content), "")
		require.ErrorIs(t, err, parentbilling.ErrImportFileInvalid, name)
	}
}

// A sequence beside the SEPA scheme reads as the sequence (#7009).
func TestImportSequenceIgnoresTheScheme(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]string{"CORE/RCUR": parentbilling.SequenceRecurring, "core frst": parentbilling.SequenceFirst,
		"RCUR": parentbilling.SequenceRecurring} {
		sequence, _, ok := importSequence(value)
		require.True(t, ok, value)
		assert.Equal(t, want, sequence, value)
	}
	for _, value := range []string{"CORE", "CORE/B2B"} {
		_, _, ok := importSequence(value)
		assert.False(t, ok, value)
	}
}

func TestSuggestImportMappingFindsTheUsualHeaders(t *testing.T) {
	t.Parallel()
	mapping := suggestImportMapping([]string{"Mandats-Referenz", "Datum der Mandatserteilung", "IBAN", "BIC", "Kontoinhaber",
		"Vertragsnummer", "Gläubiger-ID", "Lastschriftart", "Letzter Einzug"})
	assert.Equal(t, parentbilling.SEPAMandateImportColumns{Reference: "Mandats-Referenz", SignedOn: "Datum der Mandatserteilung",
		IBAN: "IBAN", BIC: "BIC", AccountHolder: "Kontoinhaber", Payer: "Vertragsnummer", CreditorID: "Gläubiger-ID",
		Sequence: "Lastschriftart", LastCollected: "Letzter Einzug"}, mapping.Columns)
	assert.Equal(t, parentbilling.PayerByContract, mapping.PayerKey, "a contract number names the payer by contract")
	assert.Equal(t, parentbilling.PayerByEmail, suggestImportMapping([]string{"E-Mail", "IBAN"}).PayerKey)
}

func TestParseImportDateReadsGermanAndISODates(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]calendar.Date{"01.02.2024": calendar.NewDate(2024, 2, 1),
		"1.2.2024": calendar.NewDate(2024, 2, 1), "01/02/2024": calendar.NewDate(2024, 2, 1), "2024-02-01": calendar.NewDate(2024, 2, 1)} {
		got, ok := parseImportDate(value)
		require.True(t, ok, value)
		assert.Equal(t, want, got, value)
	}
	// OGS Connect writes the time of day after the date (#7009).
	for value, want := range map[string]calendar.Date{"01.02.2024 14:05": calendar.NewDate(2024, 2, 1),
		"1.2.2024 7:05:09": calendar.NewDate(2024, 2, 1), "2024-02-01T00:00": calendar.NewDate(2024, 2, 1)} {
		got, ok := parseImportDate(value)
		require.True(t, ok, value)
		assert.Equal(t, want, got, value)
	}
	for _, value := range []string{"01.02.24", "31.02.2024", "gestern", "2024-13-01", "01.02.2024 25", "01.02.2024 14:05 Uhr"} {
		_, ok := parseImportDate(value)
		assert.False(t, ok, value)
	}
}

const (
	importCreditor = "DE98ZZZ09999999999"
	importIBAN     = "DE89370400440532013000"
)

var importToday = calendar.NewDate(2026, 11, 9)

// importColumnsAll maps every field to the column of the same position in
// importRow.
var importColumnsAll = importColumnIndex{payerKey: parentbilling.PayerByEmail, fields: map[string]int{
	parentbilling.ImportFieldReference: 0, parentbilling.ImportFieldSignedOn: 1, parentbilling.ImportFieldIBAN: 2,
	parentbilling.ImportFieldAccountHolder: 3, parentbilling.ImportFieldPayer: 4, parentbilling.ImportFieldCreditorID: 5,
	parentbilling.ImportFieldSequence: 6, parentbilling.ImportFieldLastCollected: 7,
}}

func importRow(reference, signed, iban, payer, creditor, sequence, lastCollected string) []string {
	return []string{reference, signed, iban, "Jürgen Müller", payer, creditor, sequence, lastCollected}
}

func newTestImportCheck(existing []parentbilling.SEPAMandate, sequences map[int]string) *importCheck {
	payers := importPayers{byContract: map[int64]importPayer{}, byFamily: map[string]importPayer{}}
	for i, family := range []string{"a@example.org", "b@example.org", "c@example.org"} {
		payer := importPayer{contractID: int64(i + 10), family: family, firstName: "Eva", lastName: "Muster"}
		payers.byContract[payer.contractID], payers.byFamily[family] = payer, payer
	}
	payers.byContract[99] = importPayer{contractID: 99}
	return newImportCheck(importColumnsAll, importCreditor, payers, existing, importToday, sequences)
}

func problemCodes(row parentbilling.SEPAMandateImportRow) []string {
	codes := []string{}
	for _, problem := range row.Problems {
		codes = append(codes, problem.Field+":"+problem.Code)
	}
	return codes
}

// A complete old mandate already collected with comes as RCUR with its last
// collection and keeps its reference and date; a mandate never collected
// with comes as FRST.
func TestImportCheckTakesOverCompleteOldMandates(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	used := check.row(importRow("SJR-1", "01.02.2024", "DE89 3704 0044 0532 0130 00", "A@Example.org ", importCreditor, "RCUR", "01.10.2026"), 2)
	assert.Equal(t, parentbilling.ImportRowReady, used.row.Status, problemCodes(used.row))
	assert.Equal(t, "SJR-1", used.row.Reference)
	assert.Equal(t, calendar.NewDate(2024, 2, 1), *used.row.SignedOn)
	assert.Equal(t, parentbilling.SequenceRecurring, used.row.Sequence)
	assert.Equal(t, calendar.NewDate(2026, 10, 1), *used.row.LastCollectedOn)
	assert.Equal(t, int64(10), used.row.ContractID)
	assert.Equal(t, "a@example.org", used.family)
	assert.Equal(t, importIBAN, used.iban)
	assert.NotContains(t, used.row.IBANMasked, "0532", "the IBAN shows only masked")
	assert.Empty(t, used.row.Hints)

	fresh := check.row(importRow("SJR-2", "2026-09-01", importIBAN, "b@example.org", "", "Erstlastschrift", ""), 3)
	assert.Equal(t, parentbilling.ImportRowInvalid, fresh.row.Status, "without the choice an empty creditor cell is missing")
	assert.Contains(t, problemCodes(fresh.row), "creditor_id:missing")

	byLastCollection := check.row(importRow("SJR-3", "2026-01-01", importIBAN, "c@example.org", importCreditor, "", "2026-10-01"), 4)
	assert.Equal(t, parentbilling.ImportRowReady, byLastCollection.row.Status, "a last collection proves the mandate was used")
	assert.Equal(t, parentbilling.SequenceRecurring, byLastCollection.row.Sequence)
}

func TestImportCheckTakesTheTraegerCreditorOnlyWhenChosen(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	check.columns.creditorFromTraeger = true
	fresh := check.row(importRow("SJR-2", "2026-09-01", importIBAN, "b@example.org", "", "FRST", ""), 3)
	assert.Equal(t, parentbilling.ImportRowReady, fresh.row.Status, problemCodes(fresh.row))
	assert.Equal(t, importCreditor, fresh.creditorID)
	assert.Equal(t, parentbilling.SequenceFirst, fresh.row.Sequence)
	other := check.row(importRow("SJR-3", "2026-09-01", importIBAN, "c@example.org", "DE98ABC09999999999", "FRST", ""), 4)
	assert.Equal(t, parentbilling.ImportRowConflict, other.row.Status, "a mandate of another creditor is never the Träger's")
	assert.Equal(t, []string{"creditor_id:creditor_mismatch"}, problemCodes(other.row))
}

// A missing or unknown sequence is never guessed: the row waits for the
// office's answer, and a used mandate without its last collection is taken
// over with a hint.
func TestImportCheckNeverGuessesTheSequence(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, map[int]string{3: parentbilling.SequenceRecurring})
	missing := check.row(importRow("SJR-1", "2026-01-01", importIBAN, "a@example.org", importCreditor, "", ""), 2)
	assert.Equal(t, parentbilling.ImportRowClarify, missing.row.Status)
	assert.Equal(t, []string{"sequence:sequence_unknown"}, problemCodes(missing.row))
	assert.Empty(t, missing.row.Sequence)

	answered := check.row(importRow("SJR-2", "2026-01-01", importIBAN, "b@example.org", importCreditor, "?", ""), 3)
	assert.Equal(t, parentbilling.ImportRowReady, answered.row.Status, problemCodes(answered.row))
	assert.Equal(t, parentbilling.SequenceRecurring, answered.row.Sequence)
	assert.Equal(t, []string{parentbilling.ImportHintLastCollectionUnknown}, answered.row.Hints)

	contradicting := check.row(importRow("SJR-3", "2026-01-01", importIBAN, "c@example.org", importCreditor, "FRST", "2026-10-01"), 4)
	assert.Equal(t, parentbilling.ImportRowClarify, contradicting.row.Status)
	assert.Equal(t, []string{"sequence:sequence_contradicts"}, problemCodes(contradicting.row))
}

// Missing fields and wrong values are named per field with the line; the
// value of an invalid IBAN is never repeated.
func TestImportCheckNamesMissingAndInvalidFields(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	empty := check.row([]string{"", "", "", "", "", "", "", ""}, 5)
	assert.Equal(t, parentbilling.ImportRowInvalid, empty.row.Status)
	assert.Equal(t, 5, empty.row.Line)
	assert.ElementsMatch(t, []string{"reference:missing", "signed_on:missing", "account_holder:missing", "iban:missing",
		"payer:missing", "creditor_id:missing", "sequence:sequence_unknown"}, problemCodes(empty.row))

	wrong := check.row(importRow("SJR/1//x", "32.01.2024", "DE89370400440532013001", "nobody@example.org", "DE00", "OOFF", "morgen"), 6)
	assert.Equal(t, parentbilling.ImportRowInvalid, wrong.row.Status)
	assert.ElementsMatch(t, []string{"reference:invalid", "signed_on:invalid", "iban:invalid", "payer:payer_unknown",
		"creditor_id:invalid", "sequence:one_off", "last_collected_on:invalid"}, problemCodes(wrong.row))
	assert.Empty(t, wrong.row.IBANMasked, "an invalid IBAN is not shown")
	assert.Empty(t, wrong.iban)

	future := check.row(importRow("SJR-F", "01.12.2026", importIBAN, "a@example.org", importCreditor, "RCUR", "01.10.2026"), 7)
	assert.ElementsMatch(t, []string{"signed_on:future"}, problemCodes(future.row))
	early := check.row(importRow("SJR-E", "01.10.2026", importIBAN, "a@example.org", importCreditor, "RCUR", "01.09.2026"), 8)
	assert.ElementsMatch(t, []string{"last_collected_on:before_signed"}, problemCodes(early.row))
	noFamily := check.row(importRow("SJR-N", "01.10.2026", importIBAN, "99", importCreditor, "FRST", ""), 9)
	assert.Equal(t, parentbilling.ImportRowInvalid, noFamily.row.Status, "payer by e-mail does not read a contract number")
	assert.Equal(t, []string{"payer:payer_unknown"}, problemCodes(noFamily.row))
}

// 36 months without a collection end a mandate: it is not taken over.
func TestImportCheckRefusesLapsedMandates(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	lapsed := check.row(importRow("SJR-L", "01.01.2020", importIBAN, "a@example.org", importCreditor, "RCUR", "09.11.2023"), 2)
	assert.Equal(t, parentbilling.ImportRowInvalid, lapsed.row.Status)
	assert.Equal(t, []string{"signed_on:expired"}, problemCodes(lapsed.row))
	held := check.row(importRow("SJR-H", "01.01.2020", importIBAN, "b@example.org", importCreditor, "RCUR", "10.11.2023"), 3)
	assert.Equal(t, parentbilling.ImportRowReady, held.row.Status, "the day before the lapse it still holds")
}

// A repeated import skips what the Träger has; a contradiction with a
// stored mandate or an earlier row is a conflict, never an overwrite.
func TestImportCheckSkipsDuplicatesAndReportsConflicts(t *testing.T) {
	t.Parallel()
	existing := []parentbilling.SEPAMandate{
		{ID: 1, Reference: "SJR-1", FamilyEmail: "a@example.org", SignedOn: calendar.NewDate(2024, 2, 1), CreditorID: importCreditor},
		{ID: 2, Reference: "SJR-2", FamilyEmail: "b@example.org", SignedOn: calendar.NewDate(2024, 2, 1), CreditorID: importCreditor},
	}
	check := newTestImportCheck(existing, nil)
	// The stored account of SJR-1 as the owner opened it; SJR-2 stays
	// unopened.
	check.accounts[existing[0].ID] = importAccount{iban: importIBAN, holder: "Jürgen Müller"}
	same := check.row(importRow("SJR-1", "01.02.2024", importIBAN, "a@example.org", importCreditor, "RCUR", ""), 2)
	assert.Equal(t, parentbilling.ImportRowDuplicate, same.row.Status)
	assert.Equal(t, existing[0].ID, same.row.MandateID)
	otherAccount := check.row(importRow("SJR-1", "01.02.2024", "DE02120300000000202051", "a@example.org", importCreditor, "RCUR", ""), 8)
	assert.Equal(t, parentbilling.ImportRowConflict, otherAccount.row.Status, "another IBAN is no duplicate")
	assert.Equal(t, []string{"iban:account_differs"}, problemCodes(otherAccount.row))
	otherHolder := importRow("SJR-1", "01.02.2024", importIBAN, "a@example.org", importCreditor, "RCUR", "")
	otherHolder[3] = "Jan Müller"
	assert.Equal(t, []string{"account_holder:account_differs"}, problemCodes(check.row(otherHolder, 9).row))
	unopened := check.row(importRow("SJR-2", "01.02.2024", importIBAN, "b@example.org", importCreditor, "RCUR", ""), 10)
	assert.Equal(t, parentbilling.ImportRowConflict, unopened.row.Status, "a mandate not compared is no duplicate")

	otherDate := check.row(importRow("SJR-2", "02.02.2024", importIBAN, "b@example.org", importCreditor, "RCUR", ""), 3)
	assert.Equal(t, parentbilling.ImportRowConflict, otherDate.row.Status)
	assert.Contains(t, problemCodes(otherDate.row), "reference:reference_taken")

	secondMandate := check.row(importRow("SJR-9", "01.02.2024", importIBAN, "a@example.org", importCreditor, "RCUR", ""), 4)
	assert.Equal(t, []string{"payer:payer_has_mandate"}, problemCodes(secondMandate.row))

	first := check.row(importRow("SJR-C", "01.02.2026", importIBAN, "c@example.org", importCreditor, "RCUR", ""), 5)
	require.Equal(t, parentbilling.ImportRowReady, first.row.Status)
	again := check.row(importRow("SJR-C", "01.02.2026", importIBAN, "c@example.org", importCreditor, "RCUR", ""), 6)
	assert.Equal(t, parentbilling.ImportRowConflict, again.row.Status)
	assert.ElementsMatch(t, []string{"reference:reference_repeated", "payer:payer_repeated"}, problemCodes(again.row))

	// A conflicting row claims nothing: a later row of that family is free.
	conflicting := check.row(importRow("SJR-1", "01.02.2026", importIBAN, "b@example.org", importCreditor, "RCUR", ""), 7)
	require.Equal(t, parentbilling.ImportRowConflict, conflicting.row.Status)
	assert.Equal(t, []string{"reference:reference_taken", "payer:payer_has_mandate"}, problemCodes(conflicting.row))
}

func TestImportFileNameKeepsOnlyTheBaseName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ogs-export.csv", importFileName(`C:\\Users\\buero\\ogs-export.csv`))
	assert.Equal(t, "ogs-export.csv", importFileName("../../ogs-export.csv"))
	assert.Equal(t, "Mandate 2026.csv", importFileName(" Mandate\t 2026.csv\n"))
	assert.Equal(t, "import.csv", importFileName(""))
	assert.Equal(t, "import.csv", importFileName("DE89370400440532013000.csv"), "an IBAN in the name never reaches the trail")
	assert.Len(t, []rune(importFileName(strings.Repeat("ä", 300))), parentbilling.MaxImportFileNameLength)
}

// The import follows the expiry rule of the debit file (#4697): a mandate
// used elsewhere whose last collection the file does not tell counts from
// the import, never from its signature, and stays RCUR; one never collected
// with counts from its signature.
func TestImportCheckUsesTheExpiryRuleOfTheDebitFile(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	used := check.row(importRow("SJR-U", "01.01.2020", importIBAN, "a@example.org", importCreditor, "RCUR", ""), 2)
	assert.Equal(t, parentbilling.ImportRowReady, used.row.Status, problemCodes(used.row))
	assert.Equal(t, parentbilling.SequenceRecurring, used.row.Sequence, "a used mandate never becomes FRST")
	assert.Nil(t, used.row.LastCollectedOn)
	assert.Equal(t, []string{parentbilling.ImportHintLastCollectionUnknown}, used.row.Hints, "an open question, not a lapse")

	mandate := parentbilling.SEPAMandate{SignedOn: *used.row.SignedOn, Kind: parentbilling.MandateRecurring,
		Sequence: used.row.Sequence, CreatedAt: importToday.BerlinMidnight()}
	assert.False(t, mandate.Expired(importToday), "imported today it holds")
	assert.Equal(t, calendar.NewDate(2029, 11, 9), mandate.ExpiryDay(), "36 months from the import")
	assert.Equal(t, parentbilling.SequenceTypeRecurring, mandate.SequenceType())

	collected := check.row(importRow("SJR-C", "01.01.2020", importIBAN, "b@example.org", importCreditor, "RCUR", "15.03.2025"), 3)
	require.Equal(t, parentbilling.ImportRowReady, collected.row.Status, problemCodes(collected.row))
	mandate.Sequence, mandate.LastCollectedOn = collected.row.Sequence, collected.row.LastCollectedOn
	assert.Equal(t, calendar.NewDate(2028, 3, 15), mandate.ExpiryDay(), "36 months from the last collection of the file")

	unused := check.row(importRow("SJR-F", "08.11.2023", importIBAN, "c@example.org", importCreditor, "FRST", ""), 4)
	assert.Equal(t, []string{"signed_on:expired"}, problemCodes(unused.row), "never collected: 36 months from the signature")
}

// A column mapped by mistake never repeats an IBAN as reference or account
// holder: the row is invalid and the value stays out of the answer.
func TestImportCheckNeverRepeatsAnIBANInATextField(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	cells := importRow(importIBAN, "01.02.2026", importIBAN, "a@example.org", importCreditor, "FRST", "")
	cells[3] = "DE89 3704 0044 0532 0130 00"
	row := check.row(cells, 2)
	assert.Equal(t, parentbilling.ImportRowInvalid, row.row.Status)
	assert.ElementsMatch(t, []string{"reference:invalid", "account_holder:invalid"}, problemCodes(row.row))
	assert.Empty(t, row.row.Reference)
	assert.Empty(t, row.row.AccountHolder)
}

func TestContainsIBANFindsAnIBANInsideText(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"DE89 3704 0044 0532 0130 00", "Eva (DE89370400440532013000)", "M-DE89370400440532013000",
		"Konto de89 3704 0044 0532 0130 00 Sparkasse", "AB12ABCDEFGHIDE89370400440532013000"} {
		assert.True(t, containsIBAN(value), value)
	}
	for _, value := range []string{"SJR-2026-0001", "Eva Beispiel", "DE89370400440532013001", "OGS-2024-0001"} {
		assert.False(t, containsIBAN(value), value)
	}
}
