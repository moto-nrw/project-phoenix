package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The DATEV file of a run holds only new bookings (#6433): what an earlier
// DATEV file held stays out unless the person chooses "all again", a run
// without a new booking makes no file and takes no number, a later Storno
// is a booking of its own fiscal year, and an invoice with a debtor DATEV
// refuses no longer locks its run once a Storno cancels it.

func datevFileForTest(t *testing.T, store *datevRunStore, options parentbilling.DatevFileOptions) (parentbilling.ExportedFile, error) {
	t.Helper()
	return newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, options)
}

func bookingNumbers(t *testing.T, data []byte) []string {
	t.Helper()
	var numbers []string
	for _, line := range datevLines(t, data)[2:] {
		numbers = append(numbers, line[10])
	}
	return numbers
}

// Invariant 5: the standard file leaves out every booking an earlier DATEV
// file held and records the ones it holds. Without the filter MB-2026-0001
// would be booked a second time.
func TestDatevFileHoldsOnlyTheBookingsNoDatevFileHeld(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[0].DatevBooked = true
	store := &datevRunStore{content: content}
	file, err := datevFileForTest(t, store, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{`"ST-2026-0001"`, `"MB-2026-0003"`}, bookingNumbers(t, file.Data))
	record := store.records[0]
	assert.False(t, record.DatevAll)
	assert.Equal(t, []parentbilling.DatevBookingRef{{CareCancellationID: 21}, {CareInvoiceID: 3}}, record.DatevBookings,
		"the Storno is a booking of its own, apart from its invoice")
	assert.Equal(t, int64(-1300), record.TotalCents, "the file counts only its own bookings")
	assert.Equal(t, 2, record.DocumentCount)
}

// Invariant 6: every booking went to DATEV before: no file, no number.
func TestDatevFileWithoutNewBookingsTakesNoNumber(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	for i := range content.Invoices {
		content.Invoices[i].DatevBooked = true
	}
	store := &datevRunStore{content: content}
	_, err := datevFileForTest(t, store, parentbilling.DatevFileOptions{})
	require.ErrorIs(t, err, parentbilling.ErrDatevNoNewBookings)
	assert.Zero(t, store.reserved, "no file number is taken")
	assert.Empty(t, store.records)
}

// "All again" repeats every booking on purpose and the protocol marks it.
func TestDatevFileOfAllBookingsAgainRepeatsThemAndSaysSo(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	for i := range content.Invoices {
		content.Invoices[i].DatevBooked = true
	}
	store := &datevRunStore{content: content}
	file, err := datevFileForTest(t, store, parentbilling.DatevFileOptions{All: true})
	require.NoError(t, err)
	assert.Equal(t, []string{`"MB-2026-0001"`, `"ST-2026-0001"`, `"MB-2026-0003"`}, bookingNumbers(t, file.Data))
	assert.True(t, store.records[0].DatevAll, "the protocol marks the repeat")
	assert.Len(t, store.records[0].DatevBookings, 3)
}

// A Storno in the next fiscal year after its invoice went to DATEV is a
// booking of its own in a file of that fiscal year: the header names the
// fiscal year 2027 and the month of the Storno, the document date 05.01.
// (DATEV writes TTMM and takes the year from the header).
func TestDatevFileBooksALaterStornoInItsOwnFiscalYear(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[0].DatevBooked, content.Invoices[3].DatevBooked = true, true
	content.Invoices[1].IssuedOn = calendar.NewDate(2027, time.January, 5)
	store := &datevRunStore{content: content}
	file, err := datevFileForTest(t, store, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	lines := datevLines(t, file.Data)
	require.Len(t, lines, 3)
	assert.Equal(t, []string{"20270101", "4", "20270101", "20270131"}, lines[0][12:16], "WJ-Beginn, Sachkontenlänge, Datum vom, Datum bis")
	assert.Equal(t, []string{"78,00", `"H"`, "10001"}, []string{lines[2][0], lines[2][1], lines[2][6]})
	assert.Equal(t, "0501", lines[2][9])
}

// One file holds one fiscal year. Unbooked documents of 2026 and a Storno of
// 2027: the file takes 2026, the Storno waits for the next file; a chosen
// fiscal year takes that year.
func TestDatevFileHoldsOneFiscalYear(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[1].IssuedOn = calendar.NewDate(2027, time.January, 5)
	store := &datevRunStore{content: content}
	file, err := datevFileForTest(t, store, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{`"MB-2026-0001"`, `"MB-2026-0003"`}, bookingNumbers(t, file.Data))
	assert.Equal(t, "20260101", datevLines(t, file.Data)[0][12])

	file, err = datevFileForTest(t, store, parentbilling.DatevFileOptions{All: true, FiscalYearStart: calendar.NewDate(2027, time.January, 1)})
	require.NoError(t, err)
	assert.Equal(t, []string{`"ST-2026-0001"`}, bookingNumbers(t, file.Data))

	_, err = datevFileForTest(t, store, parentbilling.DatevFileOptions{FiscalYearStart: calendar.NewDate(2028, time.January, 1)})
	require.ErrorIs(t, err, parentbilling.ErrDatevNoNewBookings, "a fiscal year without a booking makes no file")
}

// An invoice with a debtor DATEV refuses locked its run for good (#6427).
// Once a Storno cancels it, both are left out while neither went to DATEV,
// and the run makes its file again. Without rule 2 the file is refused.
func TestDatevFileLeavesOutACancelledInvoiceWithARefusedDebtor(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[0].DebtorNumber, content.Invoices[1].DebtorNumber = "10 001", "10 001"
	store := &datevRunStore{content: content}
	file, err := datevFileForTest(t, store, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{`"MB-2026-0003"`}, bookingNumbers(t, file.Data))

	// Without its Storno the invoice still refuses the file and is named.
	content.Invoices = append(content.Invoices[:1], content.Invoices[2:]...)
	_, err = datevFileForTest(t, &datevRunStore{content: content}, parentbilling.DatevFileOptions{})
	var refused *parentbilling.DatevBookingsError
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, "MB-2026-0001", refused.Problems[0].DocumentNumber)
}

// A holiday Storno refers to its invoice; the pair is left out the same way.
func TestDatevSelectionPairsAHolidayStornoByItsReference(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.July, 1)
	invoices := []parentbilling.ExportInvoice{
		{DocumentID: 7, Kind: "invoice", IssuedOn: day, TotalCents: 4500, DebtorNumber: "abc"},
		{DocumentID: 8, ReferenceDocumentID: 7, Kind: "cancellation", IssuedOn: day, TotalCents: -4500, DebtorNumber: "abc"},
		{DocumentID: 9, Kind: "invoice", IssuedOn: day, TotalCents: 3000, DebtorNumber: "10009"},
	}
	selection := selectDatevDocuments(invoices, datevDebtorAccount(4), time.January, false)
	require.Len(t, selection.bookable, 1)
	assert.Equal(t, int64(9), selection.bookable[0].DocumentID)

	// Once the invoice went to DATEV its Storno has to follow it.
	invoices[0].DatevBooked = true
	selection = selectDatevDocuments(invoices, datevDebtorAccount(4), time.January, false)
	assert.Len(t, selection.bookable, 2)
}

// The export page counts the new bookings by the same rules as the file and
// names the fiscal years of the run; without valid DATEV values it counts
// nothing.
func TestDatevRunOverviewsCountTheNewBookings(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[0].DatevBooked = true
	content.Invoices[1].IssuedOn = calendar.NewDate(2027, time.January, 5)
	store := &datevOverviewStore{bookings: content.Invoices}
	run := parentbilling.ExportRunRef{RunKind: parentbilling.DebitRunCare, RunID: 5}
	runs := []parentbilling.ExportRunRef{run, run, {RunKind: parentbilling.DebitRunNotice, RunID: 202610}}
	overviews, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevRunOverviews(context.Background(), sjrReviewer, runs)
	require.NoError(t, err)
	assert.True(t, overviews.Ready)
	require.Len(t, overviews.Runs, 1, "one answer per run; the Jahresbescheide have no DATEV file")
	assert.Equal(t, 2, overviews.Runs[0].NewBookings, "the Storno of 2027 and MB-2026-0003")
	assert.Equal(t, []calendar.Date{calendar.NewDate(2026, time.January, 1), calendar.NewDate(2027, time.January, 1)},
		overviews.Runs[0].FiscalYears)
	assert.Equal(t, []parentbilling.ExportRunRef{run}, store.asked)
	assert.Equal(t, sjrReviewer.OrganizationID, store.organization)

	store = &datevOverviewStore{bookings: content.Invoices}
	overviews, err = newFinanceExportsForTest(store, runPost{}).DatevRunOverviews(context.Background(), sjrReviewer, runs)
	require.NoError(t, err)
	assert.False(t, overviews.Ready)
	assert.Empty(t, overviews.Runs)
	assert.Nil(t, store.asked, "no count without valid DATEV values")
}

// datevOverviewStore answers the bookings of the runs it is asked for.
type datevOverviewStore struct {
	FinanceExportStore
	bookings     []parentbilling.ExportInvoice
	asked        []parentbilling.ExportRunRef
	organization int64
}

func (s *datevOverviewStore) DatevRunBookings(_ context.Context, organizationID int64, runs []parentbilling.ExportRunRef,
) (map[parentbilling.ExportRunRef][]parentbilling.ExportInvoice, error) {
	s.organization, s.asked = organizationID, runs
	return map[parentbilling.ExportRunRef][]parentbilling.ExportInvoice{runs[0]: s.bookings}, nil
}
