package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// sollInvoices are an invoice, its Storno and a holiday invoice whose
// recipient a spreadsheet would read as a formula.
func sollInvoices() []parentbilling.ExportInvoice {
	due := calendar.NewDate(2026, 11, 4)
	return []parentbilling.ExportInvoice{
		{CareInvoiceID: 11, Number: "MB-2026-0110", Kind: "invoice", IssuedOn: calendar.NewDate(2026, 10, 30), DueOn: &due,
			TotalCents: 7800, DebtorNumber: "10 4711", RecipientName: "Eva Müller", Street: "Sonnenweg 7", PostalCode: "75177",
			City: "Pforzheim", ChildName: "Lia Müller", SchoolName: "GS Ispringen", ServiceFrom: calendar.NewDate(2026, 10, 1),
			ServiceUntil: calendar.NewDate(2026, 10, 31), PaymentMethod: parentbilling.PaymentDirectDebit, MandateReference: "SJR-0001"},
		{CareInvoiceID: 11, Number: "MB-2026-0111", Kind: "cancellation", IssuedOn: calendar.NewDate(2026, 11, 2),
			TotalCents: -7800, DebtorNumber: "10 4711", RecipientName: "Eva Müller", ChildName: "Lia Müller",
			ServiceFrom: calendar.NewDate(2026, 10, 1), ServiceUntil: calendar.NewDate(2026, 10, 31), PaymentMethod: parentbilling.PaymentDirectDebit},
		{DocumentID: 12, Number: "FE-2026-0007", Kind: "invoice", IssuedOn: calendar.NewDate(2026, 10, 30), DueOn: &due,
			TotalCents: 123456, RecipientName: "=HYPERLINK(\"x\")", ChildName: "Ben; Ida", ServiceFrom: calendar.NewDate(2026, 10, 26),
			ServiceUntil: calendar.NewDate(2026, 10, 30), PaymentMethod: parentbilling.PaymentTransfer},
	}
}

// The Sollstellung of the default mapping and of a mapping as a Kämmerei
// asks for it, byte for byte (#4693). UPDATE_GOLDEN=1 rewrites the files.
func TestSollstellungGolden(t *testing.T) {
	t.Parallel()
	kaemmerei := parentbilling.SollstellungMapping{
		Columns: []parentbilling.SollstellungColumn{
			{Field: parentbilling.SollFieldConstant, Header: "Buchungskreis", Value: "1000"},
			{Field: parentbilling.SollFieldDebtorNumber, Header: "Personenkonto"},
			{Field: parentbilling.SollFieldInvoiceNumber, Header: "Belegnummer"},
			{Field: parentbilling.SollFieldDocumentKind, Header: "Belegart"},
			{Field: parentbilling.SollFieldInvoiceDate, Header: "Belegdatum"},
			{Field: parentbilling.SollFieldAmount, Header: "Betrag"},
			{Field: parentbilling.SollFieldRecipientName, Header: "Name"},
			{Field: parentbilling.SollFieldChildName, Header: "Kind"},
			{Field: parentbilling.SollFieldFileNumber, Header: "Datei"},
		},
		Delimiter: ",", DecimalSeparator: ".", DateFormat: parentbilling.SollDateISO, Encoding: parentbilling.SollEncodingUTF8,
		HeaderRow: true, FileNamePattern: "csv-export-bewegungsdaten-{NR}.csv",
	}
	for name, mapping := range map[string]parentbilling.SollstellungMapping{
		"default.csv":   parentbilling.DefaultSollstellungMapping(),
		"kaemmerei.csv": kaemmerei,
	} {
		normalized, err := normalizeSollMapping(mapping)
		require.NoError(t, err, name)
		data, err := renderSollRows(normalized, invoiceSollRows(sollInvoices()), 110)
		require.NoError(t, err, name)
		path := filepath.Join("testdata", "sollstellung", name)
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, data, 0o600))
		}
		golden, err := os.ReadFile(path)
		require.NoError(t, err, name)
		assert.Equal(t, string(golden), string(data), name)
	}
	assert.Equal(t, "csv-export-bewegungsdaten-110.csv", sollFileName(kaemmerei.FileNamePattern, 110))
}

// buchhaltungMapping is the plain bookkeeping file a Träger hands its tax
// office instead of a DATEV batch (#7008): nine columns, one row per line of
// where a document books, credits in a column of their own and a booking
// text with the month and the child.
func buchhaltungMapping() parentbilling.SollstellungMapping {
	return parentbilling.SollstellungMapping{
		Columns: []parentbilling.SollstellungColumn{
			{Field: parentbilling.SollFieldInvoiceNumber, Header: "Rechnungsnummer"},
			{Field: parentbilling.SollFieldInvoiceDate, Header: "Rechnungsdatum"},
			{Field: parentbilling.SollFieldAmount, Header: "Rechnungsbetrag"},
			{Field: parentbilling.SollFieldCreditAmount, Header: "Gutschrift"},
			{Field: parentbilling.SollFieldDebtorNumber, Header: "Debitor-Nr."},
			{Field: parentbilling.SollFieldRevenueAccount, Header: "Erlöskonto-Nr."},
			{Field: parentbilling.SollFieldCostCenter, Header: "KOST1"},
			{Field: parentbilling.SollFieldCostCenter2, Header: "Kostenstellen-Nr."},
			{Field: parentbilling.SollFieldBookingText, Header: "Buchungstext", Value: "SKB {Monat} {Kind}"},
		},
		Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingUTF8BOM,
		HeaderRow: true, FileNamePattern: "csv-export-bewegungsdaten-{NR}.csv",
	}
}

// The bookkeeping file of a monthly run, byte for byte (#7008): an invoice
// on two accounts, its Storno in the credit column with an empty amount, a
// holiday credit and an invoice of a sibling with a debtor number of its
// own. Invented data only. UPDATE_GOLDEN=1 rewrites the file.
func TestSollstellungBuchhaltungGolden(t *testing.T) {
	t.Parallel()
	mapping, err := normalizeSollMapping(buchhaltungMapping())
	require.NoError(t, err)
	october, until := calendar.NewDate(2026, 10, 1), calendar.NewDate(2026, 10, 31)
	parts := func(sign int64) []parentbilling.ExportBookingPart {
		return []parentbilling.ExportBookingPart{
			{AmountCents: sign * 3250, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4410", CostCenter: "100", CostCenter2: "4711", Frozen: true}},
			{AmountCents: sign * 4550, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "100", CostCenter2: "4711", Frozen: true}},
		}
	}
	invoices := []parentbilling.ExportInvoice{
		{Number: "2026-0312", Kind: parentbilling.DocumentInvoice, IssuedOn: calendar.NewDate(2026, 10, 30), TotalCents: 7800,
			DebtorNumber: "70001", ChildName: "Lia Beispiel", ServiceFrom: october, ServiceUntil: until, Parts: parts(1)},
		{Number: "2026-0313", Kind: parentbilling.DocumentInvoice, IssuedOn: calendar.NewDate(2026, 10, 30), TotalCents: 5600,
			DebtorNumber: "70002", ChildName: "Tom Beispiel", ServiceFrom: october, ServiceUntil: until,
			Parts: []parentbilling.ExportBookingPart{{AmountCents: 5600, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4400",
				CostCenter: "100", CostCenter2: "4712", Frozen: true}}}},
		{Number: "2026-0314", Kind: parentbilling.DocumentCancellation, IssuedOn: calendar.NewDate(2026, 11, 3), TotalCents: -7800,
			DebtorNumber: "70001", ChildName: "Lia Beispiel", ServiceFrom: october, ServiceUntil: until, Parts: parts(-1)},
		{Number: "2026-0315", Kind: parentbilling.DocumentCredit, IssuedOn: calendar.NewDate(2026, 11, 3), TotalCents: -1250,
			DebtorNumber: "70003", ChildName: "Ida Muster", ServiceFrom: october, ServiceUntil: until,
			Parts: []parentbilling.ExportBookingPart{{AmountCents: -1250, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4420",
				CostCenter: "200", Frozen: true}}}},
	}
	rows, _, missing := sollRows(mapping, invoices, newBookingResolver(parentbilling.BookingAssignments{}, "4000"))
	require.Empty(t, missing)
	data, err := renderSollRows(mapping, rows, 112)
	require.NoError(t, err)
	path := filepath.Join("testdata", "sollstellung", "buchhaltung.csv")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(path, data, 0o600))
	}
	golden, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(data))
	assert.True(t, strings.HasPrefix(string(data), "\xef\xbb\xbfRechnungsnummer;Rechnungsdatum;Rechnungsbetrag;Gutschrift;Debitor-Nr.;"+
		"Erlöskonto-Nr.;KOST1;Kostenstellen-Nr.;Buchungstext\r\n"), "the header of the bookkeeping file")
	assert.Equal(t, "csv-export-bewegungsdaten-112.csv", sollFileName(mapping.FileNamePattern, 112))
}

// Without a credit column a Storno keeps its negative amount, as before
// #7008; with one, the amount stays empty and the credit is positive. A
// booking text fills each placeholder and writes an empty month without a
// service date.
func TestSollstellungCreditColumnAndBookingText(t *testing.T) {
	t.Parallel()
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{
		{Field: parentbilling.SollFieldAmount},
		{Field: parentbilling.SollFieldBookingText, Value: "{Rechnungsnummer} {Empfänger}: {Kind} {Monat}"},
	}, Delimiter: ";", DecimalSeparator: ",", Encoding: parentbilling.SollEncodingUTF8}
	storno := parentbilling.ExportInvoice{Number: "S-1", Kind: parentbilling.DocumentCancellation, TotalCents: -7800,
		RecipientName: "Eva Muster", ChildName: "Lia Muster", ServiceFrom: calendar.NewDate(2026, 9, 1)}
	invoice := parentbilling.ExportInvoice{Number: "R-1", Kind: parentbilling.DocumentInvoice, TotalCents: 1000, RecipientName: "-Ben"}
	data, err := renderSollRows(mapping, invoiceSollRows([]parentbilling.ExportInvoice{storno, invoice}), 1)
	require.NoError(t, err)
	assert.Equal(t, "-78,00;S-1 Eva Muster: Lia Muster 09/2026\r\n10,00;R-1 -Ben:  \r\n", string(data))

	mapping.Columns = append(mapping.Columns, parentbilling.SollstellungColumn{Field: parentbilling.SollFieldCreditAmount})
	mapping.Columns[1].Value = "{Empfänger}"
	data, err = renderSollRows(mapping, invoiceSollRows([]parentbilling.ExportInvoice{storno, invoice}), 1)
	require.NoError(t, err)
	assert.Equal(t, ";Eva Muster;78,00\r\n10,00;'-Ben;\r\n", string(data), "a booking text that starts like a formula is text")
}

// A booking text keeps its pattern within the limits of a constant and
// refuses a placeholder it does not know; other columns drop a value.
func TestSollstellungBookingTextPattern(t *testing.T) {
	t.Parallel()
	base := parentbilling.SollstellungMapping{Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman,
		Encoding: parentbilling.SollEncodingUTF8, FileNamePattern: "s-{NR}.csv"}
	base.Columns = []parentbilling.SollstellungColumn{
		{Field: parentbilling.SollFieldBookingText, Value: " SKB {Monat} {Kind} "},
		{Field: parentbilling.SollFieldCreditAmount, Value: "lost"},
	}
	mapping, err := normalizeSollMapping(base)
	require.NoError(t, err)
	assert.Equal(t, "SKB {Monat} {Kind}", mapping.Columns[0].Value)
	assert.Empty(t, mapping.Columns[1].Value)

	var fields validation.Errors
	for _, pattern := range []string{"SKB {Monat} {Name}", strings.Repeat("x", 61), "SKB\n{Kind}"} {
		base.Columns = []parentbilling.SollstellungColumn{{Field: parentbilling.SollFieldBookingText, Value: pattern}}
		_, err = normalizeSollMapping(base)
		require.ErrorAs(t, err, &fields, pattern)
		assert.Contains(t, fields, "columns.0.value", pattern)
	}
}

// Windows-1252 writes umlauts as single bytes and replaces what it lacks.
func TestSollstellungWindows1252(t *testing.T) {
	t.Parallel()
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{{Field: parentbilling.SollFieldRecipientName}},
		Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingWindows1252,
		FileNamePattern: "s-{NR}.csv"}
	data, err := renderSollRows(mapping, invoiceSollRows([]parentbilling.ExportInvoice{{RecipientName: "Jürgen Łukasz"}}), 1)
	require.NoError(t, err)
	text, err := charmap.Windows1252.NewDecoder().Bytes(data)
	require.NoError(t, err)
	assert.Equal(t, "Jürgen ?ukasz\r\n", string(text))
}

func TestSollstellungInvoiceNumbersAreNoFormulas(t *testing.T) {
	t.Parallel()
	pattern, err := normalizeNumberPattern("-{N}-2")
	require.NoError(t, err)
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{
		{Field: parentbilling.SollFieldInvoiceNumber},
		{Field: parentbilling.SollFieldAmount},
	}, Delimiter: ";", DecimalSeparator: ",", Encoding: parentbilling.SollEncodingUTF8}
	data, err := renderSollRows(mapping, invoiceSollRows([]parentbilling.ExportInvoice{{
		Number: formatInvoiceNumber(pattern, 2026, 1), TotalCents: -7800,
	}}), 1)
	require.NoError(t, err)
	assert.Equal(t, "'-1-2;-78,00\r\n", string(data))
}

// A column header of the Träger's mapping that starts like a formula is
// written as text, as every text cell is: Excel reads "=HYPERLINK(...)" in
// a quoted CSV field as a formula too (quorum review of 1bb006b1ac).
func TestSollstellungHeadersAreNoFormulas(t *testing.T) {
	t.Parallel()
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{
		{Field: parentbilling.SollFieldInvoiceNumber, Header: "=HYPERLINK(\"https://example.org\")"},
		{Field: parentbilling.SollFieldAmount, Header: "+Betrag"},
		{Field: parentbilling.SollFieldChildName, Header: "Kind"},
	}, Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingUTF8,
		HeaderRow: true, FileNamePattern: "s-{NR}.csv"}
	data, err := renderSollRows(mapping, invoiceSollRows(nil), 1)
	require.NoError(t, err)
	assert.Equal(t, "\"'=HYPERLINK(\"\"https://example.org\"\")\";'+Betrag;Kind\r\n", string(data))
}

// A mapping names each problem at its field and keeps nothing it cannot
// write: unknown fields, a missing file number in the name, a separator
// that is also the decimal point, a header row without headers.
func TestSollstellungMappingRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()
	_, err := normalizeSollMapping(parentbilling.SollstellungMapping{
		Columns: []parentbilling.SollstellungColumn{
			{Field: "iban", Header: "IBAN"},
			{Field: parentbilling.SollFieldAmount},
			{Field: parentbilling.SollFieldConstant, Header: "Konto", Value: "1\n2"},
		},
		Delimiter: ",", DecimalSeparator: ",", DateFormat: "mm/dd/yyyy", Encoding: "latin-9", HeaderRow: true,
		FileNamePattern: "../sollstellung.csv",
	})
	var fields validation.Errors
	require.ErrorAs(t, err, &fields)
	for _, field := range []string{"columns.0.field", "columns.1.header", "columns.2.value", "decimal_separator", "date_format",
		"encoding", "file_name_pattern"} {
		assert.Contains(t, fields, field)
	}
	_, err = normalizeSollMapping(parentbilling.SollstellungMapping{Delimiter: ";", DecimalSeparator: ",",
		DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingUTF8, FileNamePattern: "s-{NR}.csv"})
	require.ErrorAs(t, err, &fields)
	assert.Contains(t, fields, "columns")
	mapping, err := normalizeSollMapping(parentbilling.SollstellungMapping{
		Columns:   []parentbilling.SollstellungColumn{{Field: parentbilling.SollFieldAmount, Header: " Betrag ", Value: "lost"}},
		Delimiter: "\t", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingUTF8,
		FileNamePattern: " s-{NR}.txt ",
	})
	require.NoError(t, err)
	assert.Equal(t, "Betrag", mapping.Columns[0].Header)
	assert.Empty(t, mapping.Columns[0].Value, "only a constant column keeps a value")
	assert.Equal(t, "s-{NR}.txt", mapping.FileNamePattern)
}

// Siblings with different debtor numbers in OGS Connect, a family with
// another number in moto, the same number again and a new one (#4693,
// decision E-c): only the new one is taken over, nothing is overwritten.
func TestImportDebtorNumbersPerFamily(t *testing.T) {
	t.Parallel()
	table := importTable{rows: [][]string{{"10 1"}, {"10 2"}, {"20 1"}, {"30 1"}, {"30 1"}, {"40 1"}, {""}, {"50 1"}}}
	columns := importColumnIndex{fields: map[string]int{parentbilling.ImportFieldDebtorNumber: 0}}
	families := []string{"geschwister@example.org", "geschwister@example.org", "anders@example.org", "neu@example.org",
		"neu@example.org", "gleich@example.org", "leer@example.org", ""}
	candidates := make([]importCandidate, len(families))
	for i, family := range families {
		candidates[i] = importCandidate{family: family, row: parentbilling.SEPAMandateImportRow{Line: i + 2, ContractID: int64(100 + i)}}
	}
	assignDebtorNumbers(candidates, table, columns, map[string]string{"anders@example.org": "20 9", "gleich@example.org": "40 1"},
		parentbilling.DebtorNumberFormat{})
	statuses := make([]string, len(candidates))
	for i, candidate := range candidates {
		statuses[i] = candidate.row.DebtorNumberStatus
	}
	assert.Equal(t, []string{parentbilling.DebtorNumberSiblingsDiffer, parentbilling.DebtorNumberSiblingsDiffer,
		parentbilling.DebtorNumberDiffers, parentbilling.DebtorNumberNew, parentbilling.DebtorNumberNew,
		parentbilling.DebtorNumberSame, "", parentbilling.DebtorNumberNoFamily}, statuses)
	assert.Equal(t, []parentbilling.FamilyDebtorNumber{{FamilyEmail: "neu@example.org", ContractID: 103, DebtorNumber: "30 1"}},
		debtorNumbersToSet(candidates), "one number per family, from its first row")
	var counts parentbilling.SEPAMandateImportCounts
	countDebtorNumbers(&counts, candidates, candidates)
	assert.Equal(t, 1, counts.DebtorNumbers)
	assert.Equal(t, 3, counts.DebtorConflicts)
}

// A sibling row with an unreadable number still disagrees: the family gets
// no number the file does not agree on (review round 2).
func TestImportDebtorNumberWithAnInvalidSibling(t *testing.T) {
	t.Parallel()
	table := importTable{rows: [][]string{{"60 1"}, {strings.Repeat("9", 40)}}}
	columns := importColumnIndex{fields: map[string]int{parentbilling.ImportFieldDebtorNumber: 0}}
	candidates := []importCandidate{
		{family: "zwei@example.org", row: parentbilling.SEPAMandateImportRow{Line: 2, ContractID: 1}},
		{family: "zwei@example.org", row: parentbilling.SEPAMandateImportRow{Line: 3, ContractID: 2}},
	}
	assignDebtorNumbers(candidates, table, columns, map[string]string{}, parentbilling.DebtorNumberFormat{})
	assert.Equal(t, parentbilling.DebtorNumberSiblingsDiffer, candidates[0].row.DebtorNumberStatus)
	assert.Equal(t, parentbilling.DebtorNumberInvalid, candidates[1].row.DebtorNumberStatus)
	assert.Empty(t, debtorNumbersToSet(candidates))
}

// A number of the file outside the Träger's format (#6675) is not taken
// over; a number in the format is.
func TestImportDebtorNumberInTheFormatOfTheTraeger(t *testing.T) {
	t.Parallel()
	table := importTable{rows: [][]string{{"20815"}, {"20 815"}}}
	columns := importColumnIndex{fields: map[string]int{parentbilling.ImportFieldDebtorNumber: 0}}
	candidates := []importCandidate{
		{family: "fuenf@example.org", row: parentbilling.SEPAMandateImportRow{Line: 2, ContractID: 1}},
		{family: "leerzeichen@example.org", row: parentbilling.SEPAMandateImportRow{Line: 3, ContractID: 2}},
	}
	assignDebtorNumbers(candidates, table, columns, map[string]string{}, parentbilling.NewDebtorNumberFormat(true, 5, 5))
	assert.Equal(t, parentbilling.DebtorNumberNew, candidates[0].row.DebtorNumberStatus)
	assert.Equal(t, parentbilling.DebtorNumberInvalid, candidates[1].row.DebtorNumberStatus)
	assert.Equal(t, []parentbilling.FamilyDebtorNumber{{FamilyEmail: "fuenf@example.org", ContractID: 1, DebtorNumber: "20815"}},
		debtorNumbersToSet(candidates))
}
