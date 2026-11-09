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

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
		data, err := renderSollstellung(normalized, sollInvoices(), 110)
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

// Windows-1252 writes umlauts as single bytes and replaces what it lacks.
func TestSollstellungWindows1252(t *testing.T) {
	t.Parallel()
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{{Field: parentbilling.SollFieldRecipientName}},
		Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingWindows1252,
		FileNamePattern: "s-{NR}.csv"}
	data, err := renderSollstellung(mapping, []parentbilling.ExportInvoice{{RecipientName: "Jürgen Łukasz"}}, 1)
	require.NoError(t, err)
	text, err := charmap.Windows1252.NewDecoder().Bytes(data)
	require.NoError(t, err)
	assert.Equal(t, "Jürgen ?ukasz\r\n", string(text))
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
	assignDebtorNumbers(candidates, table, columns, map[string]string{"anders@example.org": "20 9", "gleich@example.org": "40 1"})
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
	countDebtorNumbers(&counts, candidates)
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
	assignDebtorNumbers(candidates, table, columns, map[string]string{})
	assert.Equal(t, parentbilling.DebtorNumberSiblingsDiffer, candidates[0].row.DebtorNumberStatus)
	assert.Equal(t, parentbilling.DebtorNumberInvalid, candidates[1].row.DebtorNumberStatus)
	assert.Empty(t, debtorNumbersToSet(candidates))
}
