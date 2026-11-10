package application

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The DATEV booking batch of a released run (#4694) through the finance
// exports: Soll for an invoice, Haben for its Storno and a credit, the
// debtor of the invoice, the revenue account of the settings, no file and no
// number while the settings or a document would be refused, and every
// booked document linked to the protocol entry.

func newFinanceExportsForTest(store FinanceExportStore, post parentbilling.InvoiceHistoryPost) *FinanceExports {
	return newFinanceExportsWithDatev(store, post, &datevSettingsDouble{})
}

func newFinanceExportsWithDatev(store FinanceExportStore, post parentbilling.InvoiceHistoryPost, datev parentbilling.DatevBillingSettings,
) *FinanceExports {
	return NewFinanceExports(store, post, datev,
		func() calendar.Date { return calendar.NewDate(2026, time.November, 9) },
		func() time.Time { return time.Date(2026, time.November, 9, 10, 0, 0, 0, calendar.Berlin) })
}

// datevRunStore answers one run and records what the export reserved and
// wrote.
type datevRunStore struct {
	FinanceExportStore
	content  parentbilling.ExportRunContent
	reserved int
	records  []parentbilling.ExportFileRecord
}

func (s *datevRunStore) ExportRunContent(_ context.Context, _ int64, _ string, _ int64) (parentbilling.ExportRunContent, error) {
	return s.content, nil
}

func (s *datevRunStore) ReserveExportFileNumber(context.Context, int64) (int64, error) {
	s.reserved++
	return int64(109 + s.reserved), nil
}

func (s *datevRunStore) InsertExportFile(_ context.Context, _, _ int64, record parentbilling.ExportFileRecord,
) (parentbilling.ExportProtocolEntry, error) {
	s.records = append(s.records, record)
	return parentbilling.ExportProtocolEntry{FileNumber: record.FileNumber, Format: record.Format, FileName: record.FileName}, nil
}

func datevReadySettings() *datevSettingsDouble {
	return &datevSettingsDouble{byOrganization: map[int64]parentbilling.DatevSettings{
		sjrReviewer.OrganizationID: datevSettingsOf("29098", "55003", "4", "04", "1", "4400"),
	}}
}

func datevRunContent() parentbilling.ExportRunContent {
	october := func(day int) calendar.Date { return calendar.NewDate(2026, time.October, day) }
	month := october(1)
	return parentbilling.ExportRunContent{Run: parentbilling.ExportRun{Label: "Monatsbeiträge Oktober 2026"},
		Invoices: []parentbilling.ExportInvoice{
			{CareInvoiceID: 1, Number: "MB-2026-0001", Kind: "invoice", IssuedOn: october(30), TotalCents: 7800,
				DebtorNumber: "10001", ChildName: "Lia Müller", ServiceFrom: month},
			{CareInvoiceID: 1, CareCancellationID: 21, Number: "ST-2026-0001", Kind: "cancellation", IssuedOn: october(31), TotalCents: -7800,
				DebtorNumber: "10001", ChildName: "Lia Müller", ServiceFrom: month},
			// A full exemption moves no money and is no booking.
			{CareInvoiceID: 2, Number: "MB-2026-0002", Kind: "invoice", IssuedOn: october(30), TotalCents: 0,
				DebtorNumber: "10002", ChildName: "Ben Kaya", ServiceFrom: month},
			{CareInvoiceID: 3, Number: "MB-2026-0003", Kind: "invoice", IssuedOn: october(30), TotalCents: 6500,
				DebtorNumber: "10003", ChildName: "Ada Weiß", ServiceFrom: month},
		}}
}

func datevLines(t *testing.T, data []byte) [][]string {
	t.Helper()
	text, err := charmap.Windows1252.NewDecoder().Bytes(data)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSuffix(string(text), "\r\n"), "\r\n")
	out := make([][]string, len(lines))
	for i, line := range lines {
		out[i] = strings.Split(line, ";")
	}
	return out
}

func TestDatevFileBooksInvoicesInSollAndStornoInHaben(t *testing.T) {
	t.Parallel()
	store := &datevRunStore{content: datevRunContent()}
	file, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	require.NoError(t, err)

	assert.Equal(t, "EXTF_Buchungsstapel_110.csv", file.Entry.FileName, "the file number of the Träger names the file")
	assert.Equal(t, "text/csv; charset=windows-1252", file.ContentType)
	assert.Equal(t, "Monatsbeiträge Oktober 2026", file.Entry.RunLabel)
	lines := datevLines(t, file.Data)
	require.Len(t, lines, 5, "header, column line and three bookings; the 0 € invoice is none")
	header := lines[0]
	assert.Equal(t, []string{`"EXTF"`, "700", "21"}, header[:3])
	assert.Equal(t, []string{"29098", "55003", "20260101", "4", "20261001", "20261031", `"Beitraege Datei 110"`}, header[10:17],
		"settings, fiscal year, the month of the documents and the file number in the label")
	type booking struct{ amount, side, account, counter, date, number, text string }
	got := make([]booking, 0, 3)
	for _, line := range lines[2:] {
		got = append(got, booking{line[0], line[1], line[6], line[7], line[9], line[10], line[13]})
	}
	assert.Equal(t, []booking{
		{"78,00", `"S"`, "10001", "4400", "3010", `"MB-2026-0001"`, `"Beitrag 10/2026 Lia Müller"`},
		{"78,00", `"H"`, "10001", "4400", "3110", `"ST-2026-0001"`, `"Storno 10/2026 Lia Müller"`},
		{"65,00", `"S"`, "10003", "4400", "3010", `"MB-2026-0003"`, `"Beitrag 10/2026 Ada Weiß"`},
	}, got)

	require.Len(t, store.records, 1)
	record := store.records[0]
	assert.Equal(t, parentbilling.ExportFormatDatev, record.Format)
	assert.Equal(t, int64(110), record.FileNumber)
	assert.Equal(t, []int64{1, 3}, record.CareInvoiceIDs, "every booked invoice is in the protocol, its Storno with it")
	assert.Equal(t, 3, record.DocumentCount)
	assert.Equal(t, int64(6500), record.TotalCents, "the sum of Soll less Haben")
	assert.True(t, bytes.Equal(file.Data, record.Content), "the protocol fingerprints the file that leaves moto")
}

func TestDatevFileOfAHolidayRunCreditsTheCreditNote(t *testing.T) {
	t.Parallel()
	september := calendar.NewDate(2029, time.September, 20)
	store := &datevRunStore{content: parentbilling.ExportRunContent{Invoices: []parentbilling.ExportInvoice{
		{DocumentID: 11, Number: "FE-1", Kind: "invoice", IssuedOn: september, TotalCents: 4500, DebtorNumber: "10001",
			ChildName: "Lia Müller", ServiceFrom: september},
		{DocumentID: 12, Number: "FG-1", Kind: "credit", IssuedOn: september, TotalCents: -1500, DebtorNumber: "10001",
			ChildName: "Lia Müller", ServiceFrom: september},
	}}}
	file, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunHoliday, 5, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	lines := datevLines(t, file.Data)
	require.Len(t, lines, 4)
	assert.Equal(t, `"Ferien Datei 110"`, lines[0][16])
	assert.Equal(t, []string{"15,00", `"H"`}, lines[3][:2], "a credit relieves the debtor")
	assert.Equal(t, `"Gutschrift 09/2029 Lia Müller"`, lines[3][13])
	assert.Equal(t, []int64{11, 12}, store.records[0].DocumentIDs)
}

func TestDatevFileFingerprintsBookingsApartFromFileMetadata(t *testing.T) {
	t.Parallel()
	store := &datevRunStore{content: datevRunContent()}
	settings := datevReadySettings()
	now := time.Date(2026, time.November, 9, 10, 0, 0, 0, calendar.Berlin)
	exports := newFinanceExportsWithDatev(store, runPost{}, settings)
	exports.datev.now = func() time.Time { return now }
	export := func() parentbilling.ExportFileRecord {
		t.Helper()
		_, err := exports.DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
		require.NoError(t, err)
		return store.records[len(store.records)-1]
	}
	first := export()
	now = now.Add(time.Hour)
	again := export()
	require.NotEmpty(t, first.BookingContent)
	assert.Equal(t, first.BookingContent, again.BookingContent, "same bookings despite a new time, number and label")
	assert.NotEqual(t, first.Content, again.Content, "the audit still records each file's actual bytes")

	store.content.Invoices[3].TotalCents++
	assert.NotEqual(t, first.BookingContent, export().BookingContent, "a changed booking is not a repeat")
	store.content.Invoices[3].TotalCents--
	changed := settings.byOrganization[sjrReviewer.OrganizationID]
	changed.RevenueAccount.Value = "4410"
	settings.byOrganization[sjrReviewer.OrganizationID] = changed
	assert.NotEqual(t, first.BookingContent, export().BookingContent, "a changed counter account is not a repeat")
	changed.RevenueAccount.Value = "4400"
	changed.ClientNumber.Value = "55004"
	settings.byOrganization[sjrReviewer.OrganizationID] = changed
	assert.NotEqual(t, first.BookingContent, export().BookingContent, "the client in the header affects the booking")
}

// Invariant 1 and 3: while the check names a setting there is no file, no
// number and no protocol entry. Without the check the file would be made
// with an empty Beraternummer refused only by the formatter, after the
// number was taken.
func TestDatevFileWaitsForTheSettingsWithoutTakingANumber(t *testing.T) {
	t.Parallel()
	store := &datevRunStore{content: datevRunContent()}
	settings := &datevSettingsDouble{byOrganization: map[int64]parentbilling.DatevSettings{
		sjrReviewer.OrganizationID: datevSettingsOf("", "55003", "4", "04", "1", "44000"),
		99:                         datevSettingsOf("29098", "55003", "4", "04", "1", "4400"),
	}}
	_, err := newFinanceExportsWithDatev(store, runPost{}, settings).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	var notReady *parentbilling.DatevNotReadyError
	require.ErrorAs(t, err, &notReady)
	assert.Equal(t, []string{"consultant"}, notReady.Readiness.MissingSettings)
	assert.Equal(t, []string{"revenue"}, notReady.Readiness.InvalidSettings)
	assert.NotContains(t, settings.asked, int64(99), "only the settings of the reviewer's Träger count")
	assert.Contains(t, settings.asked, sjrReviewer.OrganizationID)
	assert.Zero(t, store.reserved, "no file number is taken")
	assert.Empty(t, store.records, "no protocol entry")
}

// Invariant 2 and 3: a document DATEV would refuse is named and refuses
// the file before a number is taken: a missing or not numeric debtor. A
// Storno in the next fiscal year is no longer refused (#6433): it waits for
// the file of its fiscal year (TestDatevFileBooksALaterStornoInItsOwnFiscalYear).
func TestDatevFileNamesTheDocumentsDatevWouldRefuse(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[3].DebtorNumber = "10 003"
	// An invoice without a debtor and without a Storno; one with a Storno
	// that never went to DATEV is left out instead (datev_run_selection.go).
	content.Invoices = append(content.Invoices, parentbilling.ExportInvoice{CareInvoiceID: 4, Number: "MB-2026-0004",
		Kind: "invoice", IssuedOn: calendar.NewDate(2026, time.October, 30), TotalCents: 7800, ServiceFrom: calendar.NewDate(2026, time.October, 1)})
	store := &datevRunStore{content: content}
	_, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	var refused *parentbilling.DatevBookingsError
	require.ErrorAs(t, err, &refused)
	assert.ElementsMatch(t, []parentbilling.DatevBookingProblem{
		{DocumentNumber: "MB-2026-0004", Field: parentbilling.DatevBookingAccount, Reason: parentbilling.DatevProblemMissing},
		{DocumentNumber: "MB-2026-0003", Field: parentbilling.DatevBookingAccount, Reason: parentbilling.DatevProblemInvalid},
	}, refused.Problems)
	assert.Zero(t, store.reserved)
	assert.Empty(t, store.records)
}

func TestDatevFileOfARunWithoutAmountsIsEmpty(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices = content.Invoices[2:3]
	store := &datevRunStore{content: content}
	_, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	require.ErrorIs(t, err, parentbilling.ErrExportRunEmpty)
	assert.Zero(t, store.reserved)

	_, err = newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunNotice, 5, parentbilling.DatevFileOptions{})
	require.ErrorIs(t, err, parentbilling.ErrExportRunNotFound, "the Jahresbescheide have no DATEV batch")
	assert.False(t, errors.Is(err, parentbilling.ErrExportRunEmpty))
}
