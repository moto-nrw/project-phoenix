package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Holiday positions book on the KOST1 of the school that hosts their week
// (#6624, invariant 5 of booking_accounts.go).

const (
	ispringen  int64 = 11
	birkenfeld int64 = 12
)

func holidayWeekOf(day calendar.Date) (*calendar.Date, *calendar.Date) {
	ends := day.AddDays(4)
	return &day, &ends
}

func holidayBase(weekID int64, day calendar.Date, cents int64) parentbilling.BillingPosition {
	starts, ends := holidayWeekOf(day)
	return parentbilling.BillingPosition{Kind: parentbilling.LineBase, AmountCents: cents, WeekID: weekID, WeekStartsOn: starts, WeekEndsOn: ends}
}

// hostAssignments: Ispringen has its KOST1 from October, Birkenfeld only
// from 19 October, the holiday care FER all year.
func hostAssignments() parentbilling.BookingAssignments {
	return parentbilling.BookingAssignments{CostCenters: []parentbilling.CostCenter{
		{Scope: parentbilling.BookingScopeSchool, ScopeID: ispringen, ValidFrom: onDay(2026, time.October, 1), Code: "ISP"},
		{Scope: parentbilling.BookingScopeSchool, ScopeID: birkenfeld, ValidFrom: onDay(2026, time.October, 19), Code: "BIR"},
		{Scope: parentbilling.BookingScopeHoliday, ValidFrom: onDay(2026, time.January, 1), Code: "FER"},
	}}
}

func TestHolidayPositionsBookOnTheKost1OfTheirHostSchool(t *testing.T) {
	t.Parallel()
	weeks := []parentbilling.HolidayWeekHost{
		{WeekID: 1, StartsOn: onDay(2026, time.October, 5), EndsOn: onDay(2026, time.October, 9), HostSchoolID: ispringen},
		{WeekID: 2, StartsOn: onDay(2026, time.October, 12), EndsOn: onDay(2026, time.October, 16), HostSchoolID: birkenfeld},
		{WeekID: 3, StartsOn: onDay(2026, time.October, 19), EndsOn: onDay(2026, time.October, 23), HostSchoolID: birkenfeld},
		// Two schools host the week of 26 October.
		{WeekID: 4, StartsOn: onDay(2026, time.October, 26), EndsOn: onDay(2026, time.October, 30), HostSchoolID: ispringen},
		{WeekID: 5, StartsOn: onDay(2026, time.October, 26), EndsOn: onDay(2026, time.October, 30), HostSchoolID: birkenfeld},
	}
	rank := 2
	documents := []parentbilling.BillingDocument{{ServiceFrom: onDay(2026, time.October, 5), Children: []parentbilling.BillingChild{
		{ChildID: 1, Positions: []parentbilling.BillingPosition{
			holidayBase(1, onDay(2026, time.October, 5), 5000),
			holidayBase(2, onDay(2026, time.October, 12), 5000),
			holidayBase(3, onDay(2026, time.October, 19), 5000),
			// A line frozen before #4699 whose week moved: matched by its days.
			holidayBase(0, onDay(2026, time.October, 19), 5000),
			// Its days name two host schools: no host school.
			holidayBase(0, onDay(2026, time.October, 26), 5000),
			// The week left the period.
			holidayBase(99, onDay(2026, time.November, 2), 5000),
		}},
		{ChildID: 2, Positions: []parentbilling.BillingPosition{
			holidayBase(3, onDay(2026, time.October, 19), 4000),
			{Kind: parentbilling.LineSibling, AmountCents: -2000, ChildRank: &rank},
			{Kind: parentbilling.LineCorrection, AmountCents: 500, Note: "Nachtrag"},
		}},
		// A child without a week (a zero exemption line).
		{ChildID: 3, Positions: []parentbilling.BillingPosition{{Kind: parentbilling.LineBenefitsExemption}}},
	}}}
	store := &bookingAccountStore{assignments: hostAssignments()}
	require.NoError(t, freezeHolidayAccounts(context.Background(), store, datevReadySettings(), sjrReviewer.OrganizationID, weeks, documents))

	type booked struct{ kost1, fallback string }
	var got [][]booked
	for _, child := range documents[0].Children {
		var positions []booked
		for _, position := range child.Positions {
			require.True(t, position.Accounts.Frozen)
			assert.Equal(t, "4400", position.Accounts.RevenueAccount, "the standard account of the DATEV settings")
			positions = append(positions, booked{position.Accounts.CostCenter, position.CostCenterFallback})
		}
		got = append(got, positions)
	}
	assert.Equal(t, [][]booked{
		{
			{"ISP", ""},
			{"FER", parentbilling.CostCenterFallbackNoSchoolCostCenter}, // Birkenfeld's KOST1 starts a week later
			{"BIR", ""},
			{"BIR", ""},
			{"FER", parentbilling.CostCenterFallbackNoHostSchool},
			{"FER", parentbilling.CostCenterFallbackNoHostSchool},
		},
		// The discount and the correction relieve the KOST1 of the child's week.
		{{"BIR", ""}, {"BIR", ""}, {"BIR", ""}},
		{{"FER", parentbilling.CostCenterFallbackNoHostSchool}},
	}, got)

	// Without a KOST1 of the holiday care the fallback stays empty and the
	// required KOST1 of the DATEV file refuses it (see below).
	documents[0].Children = documents[0].Children[:1]
	assignments := hostAssignments()
	assignments.CostCenters = assignments.CostCenters[:2]
	store = &bookingAccountStore{assignments: assignments}
	require.NoError(t, freezeHolidayAccounts(context.Background(), store, datevReadySettings(), sjrReviewer.OrganizationID, weeks, documents))
	assert.Empty(t, documents[0].Children[0].Positions[1].Accounts.CostCenter)
	assert.Equal(t, parentbilling.CostCenterFallbackNoSchoolCostCenter, documents[0].Children[0].Positions[1].CostCenterFallback)
}

// A Storno copies every position with where it booked; a credit books
// where the child's first position booked.
func TestHolidayStornoAndCreditKeepTheKost1OfTheirInvoice(t *testing.T) {
	t.Parallel()
	isp := parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "ISP", Frozen: true}
	bir := parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "BIR", Frozen: true}
	invoice := parentbilling.BillingDocument{ID: 7, Number: "FE-1", Children: []parentbilling.BillingChild{{ChildID: 1, Positions: []parentbilling.BillingPosition{
		{Kind: parentbilling.LineBase, AmountCents: 5000, Accounts: bir, CostCenterFallback: ""},
		{Kind: parentbilling.LineBase, AmountCents: 3000, Accounts: isp, CostCenterFallback: parentbilling.CostCenterFallbackNoSchoolCostCenter},
	}}}}
	storno := cancellationOf(invoice, "falsch")
	require.Len(t, storno.Children[0].Positions, 2)
	assert.Equal(t, bir, storno.Children[0].Positions[0].Accounts)
	assert.Equal(t, isp, storno.Children[0].Positions[1].Accounts)
	assert.Equal(t, parentbilling.CostCenterFallbackNoSchoolCostCenter, storno.Children[0].Positions[1].CostCenterFallback)

	credit := creditOf(invoice, invoice.Children[0], parentbilling.BillingCreditInput{AmountCents: 1000, Reason: "Tag ausgefallen"})
	require.Len(t, credit.Children[0].Positions, 1)
	assert.Equal(t, bir, credit.Children[0].Positions[0].Accounts, "the KOST1 of the child's first position")
}

func holidayPart(kost1, fallback string, cents int64, day calendar.Date) parentbilling.ExportBookingPart {
	starts, ends := holidayWeekOf(day)
	return parentbilling.ExportBookingPart{Kind: parentbilling.BookingPartHoliday, AmountCents: cents, CostCenterFallback: fallback,
		WeekStartsOn: starts, WeekEndsOn: ends, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: kost1, Frozen: true}}
}

// holidayExportContent is an invoice over two host schools with a sibling
// discount, and its Storno with every position negated.
func holidayExportContent() parentbilling.ExportRunContent {
	october := func(day int) calendar.Date { return calendar.NewDate(2026, time.October, day) }
	invoice := []parentbilling.ExportBookingPart{
		holidayPart("ISP", "", 5000, october(5)),
		holidayPart("FER", parentbilling.CostCenterFallbackNoSchoolCostCenter, 3000, october(12)),
		holidayPart("ISP", "", -800, october(5)),
	}
	storno := make([]parentbilling.ExportBookingPart, len(invoice))
	for i, part := range invoice {
		part.AmountCents = -part.AmountCents
		storno[i] = part
	}
	return parentbilling.ExportRunContent{Run: parentbilling.ExportRun{Label: "Herbstferien 2026"}, Invoices: []parentbilling.ExportInvoice{
		{DocumentID: 1, Number: "FE-2026-0001", Kind: "invoice", IssuedOn: october(1), TotalCents: 7200, DebtorNumber: "10001",
			ServiceFrom: october(5), Parts: invoice},
		{DocumentID: 2, Number: "FS-2026-0001", Kind: "cancellation", IssuedOn: october(2), TotalCents: -7200, DebtorNumber: "10001",
			ServiceFrom: october(5), Parts: storno},
	}}
}

// The DATEV file splits a holiday document onto the KOST1 of its weeks,
// the Storno relieves the same KOST1 (its negated discount weighs
// nothing), and the protocol names the positions on the fallback.
func TestDatevFileBooksHolidayWeeksOnTheirHostSchools(t *testing.T) {
	t.Parallel()
	store := &datevRunStore{content: holidayExportContent()}
	file, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunHoliday, 5, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	type booking struct{ amount, side, number, kost1 string }
	var got []booking
	for _, line := range datevLines(t, file.Data)[2:] {
		got = append(got, booking{line[0], line[1], line[10], line[36]})
	}
	assert.Equal(t, []booking{
		{"45,00", `"S"`, `"FE-2026-0001"`, `"ISP"`},
		{"27,00", `"S"`, `"FE-2026-0001"`, `"FER"`},
		{"45,00", `"H"`, `"FS-2026-0001"`, `"ISP"`},
		{"27,00", `"H"`, `"FS-2026-0001"`, `"FER"`},
	}, got)
	require.Len(t, store.records, 1)
	starts, ends := holidayWeekOf(calendar.NewDate(2026, time.October, 12))
	fallback := func(number string) parentbilling.CostCenterFallback {
		return parentbilling.CostCenterFallback{Number: number, WeekStartsOn: starts, WeekEndsOn: ends,
			Reason: parentbilling.CostCenterFallbackNoSchoolCostCenter, CostCenter: "FER"}
	}
	assert.Equal(t, []parentbilling.CostCenterFallback{fallback("FE-2026-0001"), fallback("FS-2026-0001")}, store.records[0].CostCenterFallbacks)

	// The required KOST1 stays in force: a fallback without a KOST1 of the
	// holiday care refuses the file before a number is taken.
	content := holidayExportContent()
	content.Invoices[0].Parts[1].Accounts.CostCenter = ""
	required := datevReadySettings()
	settings := required.byOrganization[sjrReviewer.OrganizationID]
	settings.CostCenterRequired = true
	required.byOrganization[sjrReviewer.OrganizationID] = settings
	store = &datevRunStore{content: content}
	_, err = newFinanceExportsWithDatev(store, runPost{}, required).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunHoliday, 5, parentbilling.DatevFileOptions{})
	var refused *parentbilling.DatevBookingsError
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, []parentbilling.DatevBookingProblem{{DocumentNumber: "FE-2026-0001", Field: parentbilling.DatevBookingCostCenter,
		Reason: parentbilling.DatevProblemMissing}}, refused.Problems)
	assert.Zero(t, store.reserved)
}

// The Sollstellung writes the same lines and names the fallback only when
// it writes the KOST1.
func TestSollstellungBooksHolidayWeeksOnTheirHostSchools(t *testing.T) {
	t.Parallel()
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{
		{Field: parentbilling.SollFieldInvoiceNumber, Header: "Nr"}, {Field: parentbilling.SollFieldAmount, Header: "Betrag"},
		{Field: parentbilling.SollFieldCostCenter, Header: "KOST1"}},
		Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingUTF8,
		HeaderRow: true, FileNamePattern: "soll-{NR}.csv"}
	invoices := holidayExportContent().Invoices
	rows, _, missing := sollRows(mapping, invoices, newBookingResolver(parentbilling.BookingAssignments{}, "4400"))
	require.Empty(t, missing)
	data, err := renderSollRows(mapping, rows, 3)
	require.NoError(t, err)
	assert.Equal(t, "Nr;Betrag;KOST1\r\nFE-2026-0001;45,00;ISP\r\nFE-2026-0001;27,00;FER\r\n"+
		"FS-2026-0001;-45,00;ISP\r\nFS-2026-0001;-27,00;FER\r\n", string(data))
	assert.Len(t, sollCostCenterFallbacks(mapping, rows), 2)

	mapping.Columns = mapping.Columns[:2]
	assert.Empty(t, sollCostCenterFallbacks(mapping, rows), "a file without KOST1 books on none")
}

// The protocol names each week of a document once; lines without a week
// (discounts, corrections) follow their child's week and add no entry,
// unless the document has no week on the fallback.
func TestCostCenterFallbacksNameEachWeekOnce(t *testing.T) {
	t.Parallel()
	october := func(day int) calendar.Date { return calendar.NewDate(2026, time.October, day) }
	reason := parentbilling.CostCenterFallbackNoSchoolCostCenter
	weekless := func(cents int64) parentbilling.ExportBookingPart {
		return parentbilling.ExportBookingPart{AmountCents: cents, CostCenterFallback: reason,
			Accounts: parentbilling.PositionAccounts{CostCenter: "FER", Frozen: true}}
	}
	invoice := parentbilling.ExportInvoice{Number: "FE-1", TotalCents: 11100, Parts: []parentbilling.ExportBookingPart{
		holidayPart("FER", reason, 5000, october(12)),
		holidayPart("FER", reason, 4000, october(12)), // a sibling in the same week
		weekless(-900),
		holidayPart("ISP", "", 3000, october(5)),
		holidayPart("FER", reason, 0, october(19)), // no amount
	}}
	starts, ends := holidayWeekOf(october(12))
	resolver := newBookingResolver(parentbilling.BookingAssignments{}, "4400")
	lines, _ := resolver.documentLines(invoice)
	assert.Equal(t, []parentbilling.CostCenterFallback{{Number: "FE-1", WeekStartsOn: starts, WeekEndsOn: ends, Reason: reason,
		CostCenter: "FER"}}, costCenterFallbacks(invoice, lines))

	only := parentbilling.ExportInvoice{Number: "FE-2", TotalCents: 1000, Parts: []parentbilling.ExportBookingPart{weekless(700), weekless(300)}}
	lines, _ = resolver.documentLines(only)
	assert.Equal(t, []parentbilling.CostCenterFallback{{Number: "FE-2", Reason: reason, CostCenter: "FER"}}, costCenterFallbacks(only, lines))
}
