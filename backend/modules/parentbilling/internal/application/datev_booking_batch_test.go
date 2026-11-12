package application

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The DATEV booking batch (#4694): fixed files under testdata/datev pin
// the header, the column line and the bookings byte for byte, in
// Windows-1252 with CR/LF. Rewrite them with
// go test ./modules/parentbilling/internal/application -run DatevBookingBatch -update-datev-batch-goldens
// and review the diff.

var updateDatevBatchGoldens = flag.Bool("update-datev-batch-goldens", false, "rewrite the DATEV booking batch golden files")

func datevTestHeader() DatevBatchHeader {
	return DatevBatchHeader{
		ConsultantNumber: 29098, ClientNumber: 55003,
		FiscalYearStart: calendar.NewDate(2026, time.January, 1), AccountLength: 4, ChartOfAccounts: "04",
		From: calendar.NewDate(2026, time.October, 1), To: calendar.NewDate(2026, time.October, 31),
		Label: "Elternbeitraege 10/2026", FileNumber: 110,
		CreatedAt: time.Date(2026, time.October, 30, 14, 4, 40, 439_000_000, calendar.Berlin),
	}
}

func datevTestBookings() []DatevBooking {
	october := func(day int) calendar.Date { return calendar.NewDate(2026, time.October, day) }
	return []DatevBooking{
		// An invoice: the debtor is charged, the revenue account credited.
		{AmountCents: 7800, Side: DatevDebit, Account: "10001", CounterAccount: "4400", DocumentDate: october(30),
			DocumentNumber: "MB-2026-0001", Text: "Monatsbeitrag Oktober Jürgen Weiß", CostCenter1: "Ispringen", CostCenter2: "Modul 2"},
		// Cents only, a short revenue account and no cost centre.
		{AmountCents: 5, Side: DatevDebit, Account: "10002", CounterAccount: "85", DocumentDate: october(1),
			DocumentNumber: "MB-2026-0002", Text: "Essen"},
		// A credit note relieves the debtor.
		{AmountCents: 1250, Side: DatevCredit, Account: "10001", CounterAccount: "4400", DocumentDate: october(31),
			DocumentNumber: "GS-2026-0001", Text: `Gutschrift "Geschwister" zu MB-2026-0001`},
		// The cancellation of an invoice relieves the debtor by the full
		// amount; the text is cut at 60 characters, a line break becomes a
		// space.
		{AmountCents: 7800, Side: DatevCredit, Account: "69999", CounterAccount: "4400", DocumentDate: october(31),
			DocumentNumber: "ST-2026-0001", Text: "Storno MB-2026-0001\nwegen falscher Anschrift der Familie Müller-Lüdenscheidt"},
		// Characters outside Windows-1252 lose their accent or become "?".
		{AmountCents: 999999999999, Side: DatevDebit, Account: "10003", CounterAccount: "4400", DocumentDate: october(15),
			DocumentNumber: "MB-2026/0003", Text: "Beitrag Ődön Wałęsa €"},
		// A character without any form in Windows-1252 is left out, never "?".
		{AmountCents: 1000, Side: DatevDebit, Account: "10004", CounterAccount: "4400", DocumentDate: october(15),
			DocumentNumber: "MB-2026-0004", Text: "Beitrag 東京 Oktober"},
	}
}

func TestDatevBookingBatchGoldens(t *testing.T) {
	t.Parallel()
	label := datevTestHeader()
	label.Label = "Elternbeitraege Oktober 2026 A" // exactly 30 characters
	label.Locked = true
	label.ChartOfAccounts = ""
	cases := []struct {
		name     string
		header   DatevBatchHeader
		bookings []DatevBooking
	}{
		{name: "invoices_credit_notes_cancellations", header: datevTestHeader(), bookings: datevTestBookings()},
		{name: "header_only_locked", header: label},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := FormatDatevBookingBatch(tc.header, tc.bookings)
			require.NoError(t, err)
			path := filepath.Join("testdata", "datev", tc.name+".csv")
			if *updateDatevBatchGoldens {
				require.NoError(t, os.WriteFile(path, file.Content, 0o600))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(want), string(file.Content), "golden %s", path)
		})
	}
}

func TestDatevBookingBatchFileShape(t *testing.T) {
	t.Parallel()
	file, err := FormatDatevBookingBatch(datevTestHeader(), datevTestBookings())
	require.NoError(t, err)
	assert.Equal(t, "EXTF_Buchungsstapel_110.csv", file.FileName)

	// Windows-1252: no BOM, ü is one byte 0xFC, every line ends in CR/LF.
	assert.False(t, bytes.HasPrefix(file.Content, []byte{0xEF, 0xBB, 0xBF}))
	assert.True(t, bytes.HasSuffix(file.Content, []byte("\r\n")))
	assert.Contains(t, string(file.Content), "Gegenkonto (ohne BU-Schl\xfcssel)")
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(file.Content)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(decoded), "\r\n"), "\r\n")
	require.Len(t, lines, 2+len(datevTestBookings()))
	assert.NotContains(t, strings.Join(lines, ""), "\n", "no bare line feed")

	header := strings.Split(lines[0], ";")
	require.Len(t, header, 31)
	assert.Equal(t, []string{`"EXTF"`, "700", "21", `"Buchungsstapel"`, "13", "20261030140440439"}, header[:6])
	assert.Equal(t, "29098", header[10], "field 11 Beraternummer")
	assert.Equal(t, "55003", header[11], "field 12 Mandantennummer")
	assert.Equal(t, "20260101", header[12], "field 13 WJ-Beginn")
	assert.Equal(t, "4", header[13], "field 14 Sachkontenlänge")
	assert.Equal(t, []string{"20261001", "20261031"}, header[14:16], "fields 15, 16 period")
	assert.Equal(t, "0", header[20], "field 21 Festschreibung")
	assert.Equal(t, `"EUR"`, header[21])
	assert.Equal(t, `"04"`, header[26], "field 27 Sachkontenrahmen")

	assert.Len(t, strings.Split(lines[1], ";"), 125)
	first := strings.Split(lines[2], ";")
	require.Len(t, first, 125)
	assert.Equal(t, "78,00", first[0])
	assert.Equal(t, `"S"`, first[1])
	assert.Equal(t, "10001", first[6])
	assert.Equal(t, "4400", first[7])
	assert.Equal(t, "3010", first[9], "Belegdatum TTMM")
	assert.Equal(t, `"MB-2026-0001"`, first[10])
	assert.Equal(t, `"Monatsbeitrag Oktober Jürgen Weiß"`, first[13])
	assert.Equal(t, []string{`"Ispringen"`, `"Modul 2"`}, first[36:38])
	assert.Equal(t, "0", first[113], "field 114 Festschreibung: empty would lock automatically")

	credit := strings.Split(lines[4], ";")
	assert.Equal(t, `"H"`, credit[1])
	assert.Equal(t, `"Gutschrift ""Geschwister"" zu MB-2026-0001"`, credit[13])
	cancellation := strings.Split(lines[5], ";")
	assert.Equal(t, `"Storno MB-2026-0001 wegen falscher Anschrift der Familie Mül"`, cancellation[13])
	foreign := strings.Split(lines[6], ";")
	assert.Equal(t, "9999999999,99", foreign[0])
	assert.Equal(t, `"Beitrag Odön Walesa €"`, foreign[13])
	other := strings.Split(lines[7], ";")
	assert.Equal(t, `"Beitrag Oktober"`, other[13])
	assert.NotContains(t, string(decoded), "?", "no character is written as a silent question mark")
}

func TestDatevBookingBatchRefusesInvalidValues(t *testing.T) {
	t.Parallel()
	valid := datevTestBookings()[0]
	cases := []struct {
		name   string
		header func(*DatevBatchHeader)
		change func(*DatevBooking)
		want   []DatevProblem
	}{
		{name: "debtor without the extra digit", change: func(b *DatevBooking) { b.Account = "1000" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAccount, Reason: DatevInvalid}}},
		{name: "debtor longer than length plus one", change: func(b *DatevBooking) { b.Account = "100001" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAccount, Reason: DatevInvalid}}},
		{name: "creditor instead of debtor", change: func(b *DatevBooking) { b.Account = "70001" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAccount, Reason: DatevInvalid}}},
		{name: "debtor fits a longer account length", header: func(h *DatevBatchHeader) { h.AccountLength = 5 },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAccount, Reason: DatevInvalid}}},
		{name: "revenue account longer than the length", change: func(b *DatevBooking) { b.CounterAccount = "44000" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldCounterAccount, Reason: DatevInvalid}}},
		{name: "revenue account zero", change: func(b *DatevBooking) { b.CounterAccount = "0000" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldCounterAccount, Reason: DatevInvalid}}},
		{name: "amount zero", change: func(b *DatevBooking) { b.AmountCents = 0 },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAmount, Reason: DatevInvalid}}},
		{name: "negative amount", change: func(b *DatevBooking) { b.AmountCents = -7800 },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAmount, Reason: DatevInvalid}}},
		{name: "amount over ten digits", change: func(b *DatevBooking) { b.AmountCents = 1_000_000_000_000 },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAmount, Reason: DatevInvalid}}},
		{name: "missing accounts", change: func(b *DatevBooking) { b.Account, b.CounterAccount = "", "" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldAccount, Reason: DatevMissing},
				{Line: 1, Field: DatevFieldCounterAccount, Reason: DatevMissing}}},
		{name: "unknown side", change: func(b *DatevBooking) { b.Side = "X" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldSide, Reason: DatevInvalid}}},
		{name: "document date after the period", change: func(b *DatevBooking) { b.DocumentDate = calendar.NewDate(2026, time.November, 1) },
			want: []DatevProblem{{Line: 1, Field: DatevFieldDocumentDate, Reason: DatevOutOfPeriod}}},
		{name: "invoice number with a space", change: func(b *DatevBooking) { b.DocumentNumber = "MB 1" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldDocumentNumber, Reason: DatevInvalid}}},
		{name: "invoice number with an umlaut", change: func(b *DatevBooking) { b.DocumentNumber = "Gebühr1" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldDocumentNumber, Reason: DatevInvalid}}},
		{name: "invoice number over 36 characters is never cut", change: func(b *DatevBooking) { b.DocumentNumber = strings.Repeat("9", 37) },
			want: []DatevProblem{{Line: 1, Field: DatevFieldDocumentNumber, Reason: DatevInvalid}}},
		{name: "missing invoice number", change: func(b *DatevBooking) { b.DocumentNumber = "" },
			want: []DatevProblem{{Line: 1, Field: DatevFieldDocumentNumber, Reason: DatevMissing}}},
		{name: "cost centre with a semicolon", change: func(b *DatevBooking) { b.CostCenter1, b.CostCenter2 = "A;B", strings.Repeat("K", 37) },
			want: []DatevProblem{{Line: 1, Field: DatevFieldCostCenter1, Reason: DatevInvalid},
				{Line: 1, Field: DatevFieldCostCenter2, Reason: DatevInvalid}}},
		{name: "settings of the Träger missing", header: func(h *DatevBatchHeader) {
			h.ConsultantNumber, h.ClientNumber, h.AccountLength, h.FiscalYearStart = 0, 0, 0, ""
		}, want: []DatevProblem{
			{Field: DatevFieldConsultantNumber, Reason: DatevMissing}, {Field: DatevFieldClientNumber, Reason: DatevMissing},
			{Field: DatevFieldAccountLength, Reason: DatevMissing}, {Field: DatevFieldFiscalYearStart, Reason: DatevMissing},
			{Line: 1, Field: DatevFieldAccount, Reason: DatevInvalid}, {Line: 1, Field: DatevFieldCounterAccount, Reason: DatevInvalid},
		}},
		{name: "header numbers out of range", header: func(h *DatevBatchHeader) {
			h.ConsultantNumber, h.ClientNumber, h.AccountLength, h.ChartOfAccounts = 1000, 100000, 9, "SKR04"
		}, want: []DatevProblem{
			{Field: DatevFieldConsultantNumber, Reason: DatevInvalid}, {Field: DatevFieldClientNumber, Reason: DatevInvalid},
			{Field: DatevFieldAccountLength, Reason: DatevInvalid}, {Field: DatevFieldChartOfAccounts, Reason: DatevInvalid},
			{Line: 1, Field: DatevFieldAccount, Reason: DatevInvalid}, {Line: 1, Field: DatevFieldCounterAccount, Reason: DatevInvalid},
		}},
		{name: "period across two fiscal years", header: func(h *DatevBatchHeader) {
			h.From, h.To = calendar.NewDate(2026, time.December, 1), calendar.NewDate(2027, time.January, 31)
		}, change: func(b *DatevBooking) { b.DocumentDate = calendar.NewDate(2026, time.December, 1) },
			want: []DatevProblem{{Field: DatevFieldPeriod, Reason: DatevOutOfPeriod}}},
		{name: "period ends before it starts", header: func(h *DatevBatchHeader) { h.To = calendar.NewDate(2026, time.September, 30) },
			change: func(b *DatevBooking) { b.DocumentDate = calendar.NewDate(2026, time.October, 1) },
			want: []DatevProblem{{Field: DatevFieldPeriod, Reason: DatevInvalid},
				{Line: 1, Field: DatevFieldDocumentDate, Reason: DatevOutOfPeriod}}},
		{name: "label with a colon", header: func(h *DatevBatchHeader) { h.Label = "Beiträge: Oktober" },
			want: []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}},
		{name: "label with characters outside Windows-1252", header: func(h *DatevBatchHeader) { h.Label = "Beitraege 東京" },
			want: []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}},
		{name: "label is never shortened", header: func(h *DatevBatchHeader) { h.Label = strings.Repeat("A", 31) },
			want: []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}},
		{name: "label with a superscript two", header: func(h *DatevBatchHeader) { h.Label = "Beitrag ²" },
			want: []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}},
		{name: "label with a letter that a name would replace", header: func(h *DatevBatchHeader) { h.Label = "Beitrag ł" },
			want: []DatevProblem{{Field: DatevFieldLabel, Reason: DatevInvalid}}},
		{name: "no file number and no time", header: func(h *DatevBatchHeader) { h.FileNumber, h.CreatedAt = 0, time.Time{} },
			want: []DatevProblem{{Field: DatevFieldFileNumber, Reason: DatevMissing}, {Field: DatevFieldCreatedAt, Reason: DatevMissing}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			header, booking := datevTestHeader(), valid
			if tc.header != nil {
				tc.header(&header)
			}
			if tc.change != nil {
				tc.change(&booking)
			}
			file, err := FormatDatevBookingBatch(header, []DatevBooking{booking})
			var formatErr *DatevFormatError
			require.True(t, errors.As(err, &formatErr), "want a format error, got %v", err)
			assert.Equal(t, tc.want, formatErr.Problems)
			assert.Empty(t, file.Content, "a refused batch writes no file")
		})
	}
}

// A locked batch writes 1 into field 114 of every booking as well as into
// header field 21; an open one writes 0 in both.
func TestDatevBookingBatchFestschreibung(t *testing.T) {
	t.Parallel()
	for _, locked := range []bool{false, true} {
		header := datevTestHeader()
		header.Locked = locked
		file, err := FormatDatevBookingBatch(header, datevTestBookings()[:1])
		require.NoError(t, err)
		lines := strings.Split(string(file.Content), "\r\n")
		want := map[bool]string{false: "0", true: "1"}[locked]
		assert.Equal(t, want, strings.Split(lines[0], ";")[20], "header field 21, locked %v", locked)
		assert.Equal(t, want, strings.Split(lines[2], ";")[113], "booking field 114, locked %v", locked)
	}
}

// Invariant 4: the encoder writes strict Windows-1252 and refuses a
// character without a code instead of replacing it.
func TestDatevEncodeRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"Beitrag 東京", "Wałęsa", "Ődön", "😀"} {
		_, ok := datevEncode(text)
		assert.False(t, ok, "%q", text)
	}
	encoded, ok := datevEncode("Weiß € ²")
	require.True(t, ok)
	assert.Equal(t, []byte("Wei\xdf \x80 \xb2"), encoded)
}
