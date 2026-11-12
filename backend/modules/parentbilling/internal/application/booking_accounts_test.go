package application

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The revenue accounts and cost centers (#4821): what the page saves, how a
// position finds its accounts, how a document splits into lines, and that
// a frozen position keeps its values.

// bookingAccountStore keeps the accounts in memory.
type bookingAccountStore struct {
	assignments parentbilling.BookingAssignments
	offerings   []parentbilling.BookingOffering
}

func (s *bookingAccountStore) BookingAssignments(context.Context, int64) (parentbilling.BookingAssignments, error) {
	return s.assignments, nil
}

func (s *bookingAccountStore) BookingOfferings(context.Context, int64) ([]parentbilling.BookingOffering, error) {
	return s.offerings, nil
}

func (s *bookingAccountStore) SetRevenueAccount(_ context.Context, _, _ int64, scope string, scopeID int64, account string) error {
	s.assignments.RevenueAccounts = slices.DeleteFunc(s.assignments.RevenueAccounts, func(a parentbilling.RevenueAccount) bool {
		return a.Scope == scope && a.ScopeID == scopeID
	})
	s.assignments.RevenueAccounts = append(s.assignments.RevenueAccounts, parentbilling.RevenueAccount{Scope: scope, ScopeID: scopeID, Account: account})
	return nil
}

func (s *bookingAccountStore) DeleteRevenueAccount(_ context.Context, _ int64, scope string, scopeID int64) error {
	s.assignments.RevenueAccounts = slices.DeleteFunc(s.assignments.RevenueAccounts, func(a parentbilling.RevenueAccount) bool {
		return a.Scope == scope && a.ScopeID == scopeID
	})
	return nil
}

func (s *bookingAccountStore) InsertCostCenter(_ context.Context, _, _ int64, center parentbilling.CostCenter) error {
	center.ID = int64(len(s.assignments.CostCenters) + 1)
	s.assignments.CostCenters = append(s.assignments.CostCenters, center)
	return nil
}

func (s *bookingAccountStore) DeleteCostCenter(_ context.Context, _, id int64) error {
	before := len(s.assignments.CostCenters)
	s.assignments.CostCenters = slices.DeleteFunc(s.assignments.CostCenters, func(c parentbilling.CostCenter) bool { return c.ID == id })
	if len(s.assignments.CostCenters) == before {
		return parentbilling.ErrCostCenterNotFound
	}
	return nil
}

// traegerSchools answers the schools of the Träger 7.
type traegerSchools []parentbilling.CareSchool

func (s traegerSchools) TraegerSchools(context.Context, int64) ([]parentbilling.CareSchool, error) {
	return s, nil
}

func newBookingAccountsForTest(store *bookingAccountStore) *BookingAccounts {
	return NewBookingAccounts(store, datevReadySettings(), traegerSchools{{ID: 11, Name: "GS Ispringen"}, {ID: 12, Name: "GS Birkenfeld"}})
}

func onDay(year int, month time.Month, d int) calendar.Date { return calendar.NewDate(year, month, d) }

// Invariant 4: the page stores a value only when the DATEV file takes it,
// except a revenue account longer than the Sachkonten (a rule of two
// values, named by the file). One list, backend/testdata/finance-booking-accounts-cases.json,
// runs through the save path of the page and through the DATEV file.
func TestBookingAccountsSaveMatchesTheExportCases(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../../testdata/finance-booking-accounts-cases.json")
	require.NoError(t, err)
	var list struct {
		Cases []struct {
			Kind      string `json:"kind"`
			Value     string `json:"value"`
			Save      bool   `json:"save"`
			Export    string `json:"export"`
			DependsOn string `json:"depends_on"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	require.NotEmpty(t, list.Cases)
	kinds := map[string]bool{}
	for _, c := range list.Cases {
		kinds[c.Kind] = true
		store := &bookingAccountStore{}
		accounts := newBookingAccountsForTest(store)
		booking := DatevBooking{AmountCents: 7800, Side: DatevDebit, Account: "10001", CounterAccount: "4400",
			DocumentDate: onDay(2026, time.October, 15), DocumentNumber: "MB-2026-0001", Text: "Beitrag"}
		field := DatevFieldCounterAccount
		var saved string
		switch c.Kind {
		case "revenue_account":
			_, err = accounts.SetRevenueAccount(context.Background(), sjrReviewer, parentbilling.BookingScopeCare, 0, c.Value)
			if len(store.assignments.RevenueAccounts) == 1 {
				saved = store.assignments.RevenueAccounts[0].Account
			}
			booking.CounterAccount = saved
		case "cost_center":
			_, err = accounts.AddCostCenter(context.Background(), sjrReviewer, parentbilling.CostCenter{
				Scope: parentbilling.BookingScopeSchool, ScopeID: 11, ValidFrom: onDay(2026, time.September, 1), Code: c.Value})
			if len(store.assignments.CostCenters) == 1 {
				saved = store.assignments.CostCenters[0].Code
			}
			booking.CostCenter1, field = saved, DatevFieldCostCenter1
		default:
			t.Fatalf("unknown kind %q", c.Kind)
		}
		assert.Equal(t, c.Save, err == nil, "%s %q saved", c.Kind, c.Value)
		if !c.Save {
			var fields interface{ Error() string }
			require.ErrorAs(t, err, &fields)
			continue
		}
		_, formatErr := FormatDatevBookingBatch(datevTestHeader(), []DatevBooking{booking})
		verdict := "ok"
		var problems *DatevFormatError
		if errors.As(formatErr, &problems) && slices.ContainsFunc(problems.Problems, func(p DatevProblem) bool { return p.Field == field }) {
			verdict = "invalid"
		}
		assert.Equal(t, c.Export, verdict, "%s %q (saved %q) in the DATEV file", c.Kind, c.Value, saved)
		if verdict != "ok" {
			assert.NotEmpty(t, c.DependsOn, "%s %q is stored but refused by the export without depending on another value", c.Kind, c.Value)
		}
	}
	assert.Equal(t, map[string]bool{"revenue_account": true, "cost_center": true}, kinds)
}

// Invariant 2: a cost center holds from its day until the next row of its
// scope; an empty code ends it, and a day before the first row has none.
func TestCostCenterHoldsOnlyInItsPeriod(t *testing.T) {
	t.Parallel()
	school := parentbilling.BookingScopeSchool
	resolver := newBookingResolver(parentbilling.BookingAssignments{CostCenters: []parentbilling.CostCenter{
		{Scope: school, ScopeID: 11, ValidFrom: onDay(2027, time.January, 1), Code: "K2"},
		{Scope: school, ScopeID: 11, ValidFrom: onDay(2026, time.September, 1), Code: "K1"},
		{Scope: school, ScopeID: 11, ValidFrom: onDay(2027, time.March, 1), Code: ""},
		{Scope: school, ScopeID: 12, ValidFrom: onDay(2026, time.January, 1), Code: "B1"},
		{Scope: parentbilling.BookingScopeOffering, ScopeID: 501, ValidFrom: onDay(2026, time.October, 1), Code: "Frueh"},
	}}, "4400")
	for on, want := range map[calendar.Date]string{
		onDay(2026, time.August, 31): "", onDay(2026, time.September, 1): "K1", onDay(2026, time.December, 31): "K1",
		onDay(2027, time.January, 1): "K2", onDay(2027, time.February, 28): "K2", onDay(2027, time.March, 1): "",
	} {
		assert.Equal(t, want, resolver.care(11, 501, on).CostCenter, "KOST1 of school 11 on %s", on)
	}
	assert.Equal(t, "B1", resolver.care(12, 501, onDay(2026, time.October, 1)).CostCenter, "each school has its own")
	assert.Equal(t, "", resolver.care(11, 501, onDay(2026, time.September, 1)).CostCenter2, "KOST2 of the Angebot starts in October")
	assert.Equal(t, "Frueh", resolver.care(11, 501, onDay(2026, time.October, 1)).CostCenter2)
	assert.Equal(t, "K1", resolver.meal(11, onDay(2026, time.October, 1)).CostCenter, "the Mensa books on the school's KOST1")
	assert.Equal(t, "", resolver.holiday(onDay(2026, time.October, 1)).CostCenter, "the holiday care has its own scope")
}

func TestRevenueAccountFallsBackFromAngebotToLeistungToStandard(t *testing.T) {
	t.Parallel()
	resolver := newBookingResolver(parentbilling.BookingAssignments{RevenueAccounts: []parentbilling.RevenueAccount{
		{Scope: parentbilling.BookingScopeOffering, ScopeID: 501, Account: "4410"},
		{Scope: parentbilling.BookingScopeCare, Account: "4400"},
		{Scope: parentbilling.BookingScopeHoliday, Account: "4420"},
	}}, "4000")
	on := onDay(2026, time.October, 1)
	assert.Equal(t, "4410", resolver.care(11, 501, on).RevenueAccount, "the Angebot's own account")
	assert.Equal(t, "4400", resolver.care(11, 502, on).RevenueAccount, "the Leistung's account")
	assert.Equal(t, "4000", resolver.meal(11, on).RevenueAccount, "the standard of the DATEV settings")
	assert.Equal(t, "4420", resolver.holiday(on).RevenueAccount)
	assert.True(t, resolver.care(11, 501, on).Frozen, "the release copies what it resolved")
}

// Invariant 1: a position its release froze keeps its values, whatever the
// Träger keeps today; only a position without them takes today's values,
// and the file counts it.
func TestAFrozenPositionNeverTakesTodaysAccounts(t *testing.T) {
	t.Parallel()
	today := newBookingResolver(parentbilling.BookingAssignments{
		RevenueAccounts: []parentbilling.RevenueAccount{{Scope: parentbilling.BookingScopeCare, Account: "4999"}},
		CostCenters: []parentbilling.CostCenter{
			{Scope: parentbilling.BookingScopeSchool, ScopeID: 11, ValidFrom: onDay(2026, time.January, 1), Code: "NEU"}},
	}, "4000")
	frozen := parentbilling.ExportBookingPart{Kind: parentbilling.BookingPartCare, SchoolID: 11, CareOfferingID: 501,
		Day: onDay(2026, time.October, 1), AmountCents: 7800,
		Accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "ALT", Frozen: true}}
	// Frozen without a cost center: the school had none at the release.
	frozenBare := frozen
	frozenBare.Accounts = parentbilling.PositionAccounts{RevenueAccount: "4400", Frozen: true}
	legacy := frozen
	legacy.Accounts = parentbilling.PositionAccounts{}

	lines, resolved := today.documentLines(parentbilling.ExportInvoice{TotalCents: 7800, Parts: []parentbilling.ExportBookingPart{frozen}})
	assert.Equal(t, []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "ALT"}, amountCents: 7800}}, lines)
	assert.Zero(t, resolved)
	lines, resolved = today.documentLines(parentbilling.ExportInvoice{TotalCents: 7800, Parts: []parentbilling.ExportBookingPart{frozenBare}})
	assert.Equal(t, "", lines[0].accounts.CostCenter, "an empty frozen KOST1 stays empty")
	assert.Zero(t, resolved)
	lines, resolved = today.documentLines(parentbilling.ExportInvoice{TotalCents: 7800, Parts: []parentbilling.ExportBookingPart{legacy}})
	assert.Equal(t, parentbilling.PositionAccounts{RevenueAccount: "4999", CostCenter: "NEU"}, lines[0].accounts,
		"a position issued before #4821 books on the values of the day of the export")
	assert.Equal(t, 1, resolved)
}

// The lines of a document sum to its total; reductions, corrections and
// cost shares, which are no positions, fall on the groups in proportion.
func TestDocumentLinesSplitTheTotalByWhereThePositionsBook(t *testing.T) {
	t.Parallel()
	resolver := newBookingResolver(parentbilling.BookingAssignments{}, "4000")
	part := func(account, kost string, cents int64) parentbilling.ExportBookingPart {
		return parentbilling.ExportBookingPart{Kind: parentbilling.BookingPartCare, AmountCents: cents,
			Accounts: parentbilling.PositionAccounts{RevenueAccount: account, CostCenter: kost, Frozen: true}}
	}
	// Module 1 (6,50 × 5) and module 2 (78,00) on two accounts, the Mensa
	// (40,00) on a third; the sibling reduction left 100,00 of 150,50.
	invoice := parentbilling.ExportInvoice{TotalCents: 10000, Parts: []parentbilling.ExportBookingPart{
		part("4410", "ISP", 3250), part("4400", "ISP", 7800), part("4420", "ISP", 4000)}}
	lines, _ := resolver.documentLines(invoice)
	require.Len(t, lines, 3)
	var sum int64
	for _, line := range lines {
		sum += line.amountCents
	}
	assert.Equal(t, int64(10000), sum, "the lines sum to the total")
	assert.Equal(t, []int64{2159, 5183, 2658}, []int64{lines[0].amountCents, lines[1].amountCents, lines[2].amountCents})

	// Two positions on the same account make one line.
	lines, _ = resolver.documentLines(parentbilling.ExportInvoice{TotalCents: 11050, Parts: []parentbilling.ExportBookingPart{
		part("4400", "ISP", 3250), part("4400", "ISP", 7800)}})
	assert.Equal(t, []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "ISP"}, amountCents: 11050}}, lines)

	// A monthly Storno relieves the lines of its invoice.
	invoice.TotalCents = -10000
	lines, _ = resolver.documentLines(invoice)
	assert.Equal(t, []int64{-2159, -5183, -2658}, []int64{lines[0].amountCents, lines[1].amountCents, lines[2].amountCents})

	// A holiday Storno has only negative positions.
	lines, _ = resolver.documentLines(parentbilling.ExportInvoice{TotalCents: -4500, Parts: []parentbilling.ExportBookingPart{
		part("4420", "FER", -5000), part("4420", "FER", 500)}})
	assert.Equal(t, []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4420", CostCenter: "FER"}, amountCents: -4500}}, lines)

	// Without a position the total books on the standard.
	lines, resolved := resolver.documentLines(parentbilling.ExportInvoice{TotalCents: 500})
	assert.Equal(t, []bookingLine{{accounts: parentbilling.PositionAccounts{RevenueAccount: "4000"}, amountCents: 500}}, lines)
	assert.Equal(t, 1, resolved)
}

func TestSplitCentsKeepsEveryCent(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		total   int64
		weights []int64
	}{
		{1, []int64{1, 1, 1}}, {2, []int64{1, 1, 1}}, {100, []int64{3, 3, 3}}, {-100, []int64{3, 3, 3}},
		{999999999999, []int64{999999999999, 1}}, {7, []int64{5}},
	} {
		shares := splitCents(c.total, c.weights)
		var sum int64
		for _, share := range shares {
			sum += share
		}
		assert.Equal(t, c.total, sum, "%d over %v", c.total, c.weights)
	}
	assert.Equal(t, []int64{34, 33, 33}, splitCents(100, []int64{1, 1, 1}), "a rounding cent goes to the earlier group on a tie")
}

// The release copies where every position books: monthly positions by
// Angebot, school and first service day, the Mensa by school and month.
func TestFreezeCareAccountsCopiesWhereEachPositionBooks(t *testing.T) {
	t.Parallel()
	resolver := newBookingResolver(parentbilling.BookingAssignments{
		RevenueAccounts: []parentbilling.RevenueAccount{{Scope: parentbilling.BookingScopeOffering, ScopeID: 501, Account: "4410"},
			{Scope: parentbilling.BookingScopeMeals, Account: "4420"}},
		CostCenters: []parentbilling.CostCenter{{Scope: parentbilling.BookingScopeSchool, ScopeID: 11, ValidFrom: onDay(2026, time.October, 1), Code: "ISP"}},
	}, "4400")
	invoices := []parentbilling.CareInvoice{{SchoolID: 11,
		Positions: []parentbilling.CarePosition{
			{CareOfferingID: 501, ServiceFrom: onDay(2026, time.October, 1)},
			{CareOfferingID: 502, ServiceFrom: onDay(2026, time.September, 1)}},
		MealPositions: []parentbilling.CareMealPosition{{SchoolID: 11, MealMonth: onDay(2026, time.October, 1)}}}}
	freezeCareAccounts(resolver, invoices)
	assert.Equal(t, parentbilling.PositionAccounts{RevenueAccount: "4410", CostCenter: "ISP", Frozen: true}, invoices[0].Positions[0].Accounts)
	assert.Equal(t, parentbilling.PositionAccounts{RevenueAccount: "4400", Frozen: true}, invoices[0].Positions[1].Accounts,
		"September lies before the cost center")
	assert.Equal(t, parentbilling.PositionAccounts{RevenueAccount: "4420", CostCenter: "ISP", Frozen: true}, invoices[0].MealPositions[0].Accounts)

	week := onDay(2026, time.October, 12)
	documents := []parentbilling.BillingDocument{{ServiceFrom: week, Children: []parentbilling.BillingChild{{ChildID: 1,
		Positions: []parentbilling.BillingPosition{{Kind: parentbilling.LineBase, AmountCents: 5000, WeekStartsOn: &week, WeekEndsOn: &week}}}}}}
	store := &bookingAccountStore{assignments: parentbilling.BookingAssignments{CostCenters: []parentbilling.CostCenter{
		{Scope: parentbilling.BookingScopeHoliday, ValidFrom: onDay(2026, time.October, 12), Code: "FER"}}}}
	require.NoError(t, freezeHolidayAccounts(context.Background(), store, datevReadySettings(), sjrReviewer.OrganizationID, nil, documents))
	assert.Equal(t, parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "FER", Frozen: true}, documents[0].Children[0].Positions[0].Accounts,
		"the standard of the DATEV settings and the KOST1 of the holiday care")
	// A Storno or credit relieves the accounts of its invoice.
	documents[0].Children[0].Positions[0].Accounts.RevenueAccount = "4430"
	assert.Equal(t, "4430", cancellationOf(documents[0], "falsch").Children[0].Positions[0].Accounts.RevenueAccount)
}

func TestBookingAccountsChecksTheScopeOfTheTraeger(t *testing.T) {
	t.Parallel()
	store := &bookingAccountStore{offerings: []parentbilling.BookingOffering{
		{SchoolID: 11, CareOfferingID: 501}, {SchoolID: 99, CareOfferingID: 901}}}
	accounts := newBookingAccountsForTest(store)
	ctx := context.Background()
	_, err := accounts.SetRevenueAccount(ctx, sjrReviewer, parentbilling.BookingScopeOffering, 501, "4410")
	require.NoError(t, err)
	_, err = accounts.SetRevenueAccount(ctx, sjrReviewer, parentbilling.BookingScopeOffering, 901, "4410")
	assert.ErrorIs(t, err, parentbilling.ErrBookingScopeNotFound, "an Angebot of a school of another Träger")
	_, err = accounts.SetRevenueAccount(ctx, sjrReviewer, parentbilling.BookingScopeSchool, 11, "4410")
	assert.ErrorIs(t, err, parentbilling.ErrBookingScopeNotFound, "a school has a cost center, no revenue account")
	_, err = accounts.AddCostCenter(ctx, sjrReviewer, parentbilling.CostCenter{Scope: parentbilling.BookingScopeSchool, ScopeID: 99,
		ValidFrom: onDay(2026, time.October, 1), Code: "X"})
	assert.ErrorIs(t, err, parentbilling.ErrBookingScopeNotFound)
	_, err = accounts.AddCostCenter(ctx, sjrReviewer, parentbilling.CostCenter{Scope: parentbilling.BookingScopeCare,
		ValidFrom: onDay(2026, time.October, 1), Code: "X"})
	assert.ErrorIs(t, err, parentbilling.ErrBookingScopeNotFound, "KOST1 belongs to a school or the holiday care")
	page, err := accounts.AddCostCenter(ctx, sjrReviewer, parentbilling.CostCenter{Scope: parentbilling.BookingScopeHoliday,
		ValidFrom: onDay(2026, time.October, 1), Code: "FER"})
	require.NoError(t, err)
	assert.Equal(t, "4400", page.StandardAccount)
	assert.Len(t, page.CostCenters, 1)
	assert.Equal(t, []parentbilling.BookingOffering{{SchoolID: 11, CareOfferingID: 501}}, page.Offerings,
		"only the Angebote of the Träger's schools")
	_, err = accounts.SetRevenueAccount(ctx, sjrReviewer, parentbilling.BookingScopeOffering, 501, "")
	require.NoError(t, err)
	assert.Empty(t, store.assignments.RevenueAccounts, "an empty account removes it")
}

// Invariant 3 and 7 of the DATEV file: every line books on the account its
// position froze, with its cost centers; a line without a revenue account,
// or without a KOST1 the Träger requires, refuses the file and names the
// document.
func TestDatevFileBooksEachLineOnItsAccountAndCostCenters(t *testing.T) {
	t.Parallel()
	part := func(account, kost1, kost2 string, cents int64) parentbilling.ExportBookingPart {
		return parentbilling.ExportBookingPart{Kind: parentbilling.BookingPartCare, SchoolID: 11, AmountCents: cents,
			Accounts: parentbilling.PositionAccounts{RevenueAccount: account, CostCenter: kost1, CostCenter2: kost2, Frozen: true}}
	}
	content := datevRunContent()
	content.Invoices[0].Parts = []parentbilling.ExportBookingPart{part("4410", "ISP", "Frueh", 3250), part("4400", "ISP", "", 4550)}
	content.Invoices[1].Parts = content.Invoices[0].Parts
	content.Invoices[3].Parts = []parentbilling.ExportBookingPart{part("4400", "", "", 6500)}
	store := &datevRunStore{content: content}
	file, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	type booking struct{ amount, side, counter, number, kost1, kost2 string }
	var got []booking
	for _, line := range datevLines(t, file.Data)[2:] {
		got = append(got, booking{line[0], line[1], line[7], line[10], line[36], line[37]})
	}
	assert.Equal(t, []booking{
		{"32,50", `"S"`, "4410", `"MB-2026-0001"`, `"ISP"`, `"Frueh"`},
		{"45,50", `"S"`, "4400", `"MB-2026-0001"`, `"ISP"`, `""`},
		{"32,50", `"H"`, "4410", `"ST-2026-0001"`, `"ISP"`, `"Frueh"`},
		{"45,50", `"H"`, "4400", `"ST-2026-0001"`, `"ISP"`, `""`},
		{"65,00", `"S"`, "4400", `"MB-2026-0003"`, `""`, `""`},
	}, got)
	require.Len(t, store.records, 1)
	assert.Equal(t, 3, store.records[0].DocumentCount, "the file counts documents, not lines")
	assert.Len(t, store.records[0].DatevBookings, 3)

	// The Träger requires a KOST1: the invoice without one is named, no
	// number is taken.
	required := datevReadySettings()
	settings := required.byOrganization[sjrReviewer.OrganizationID]
	settings.CostCenterRequired = true
	required.byOrganization[sjrReviewer.OrganizationID] = settings
	store = &datevRunStore{content: content}
	_, err = newFinanceExportsWithDatev(store, runPost{}, required).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	var refused *parentbilling.DatevBookingsError
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, []parentbilling.DatevBookingProblem{{DocumentNumber: "MB-2026-0003", Field: parentbilling.DatevBookingCostCenter,
		Reason: parentbilling.DatevProblemMissing}}, refused.Problems)
	assert.Zero(t, store.reserved, "no number for a refused file")

	// A frozen account longer than the Sachkonten is named once per document.
	content.Invoices[3].Parts = []parentbilling.ExportBookingPart{part("44000", "", "", 3000), part("44001", "", "", 3500)}
	store = &datevRunStore{content: content}
	_, err = newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, []parentbilling.DatevBookingProblem{{DocumentNumber: "MB-2026-0003", Field: parentbilling.DatevBookingRevenueAccount,
		Reason: parentbilling.DatevProblemInvalid}}, refused.Problems)
	assert.Zero(t, store.reserved)
}

// A position without values of its release takes those of the day of the
// export, and the protocol counts it.
func TestDatevFileCountsThePositionsResolvedAtExport(t *testing.T) {
	t.Parallel()
	content := datevRunContent()
	content.Invoices[3].Parts = []parentbilling.ExportBookingPart{{Kind: parentbilling.BookingPartCare, SchoolID: 11, CareOfferingID: 501,
		Day: onDay(2026, time.October, 1), AmountCents: 6500}}
	store := &datevRunStore{content: content, assignments: parentbilling.BookingAssignments{CostCenters: []parentbilling.CostCenter{
		{Scope: parentbilling.BookingScopeSchool, ScopeID: 11, ValidFrom: onDay(2026, time.January, 1), Code: "ISP"}}}}
	file, err := newFinanceExportsWithDatev(store, runPost{}, datevReadySettings()).
		DatevFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 5, parentbilling.DatevFileOptions{})
	require.NoError(t, err)
	lines := datevLines(t, file.Data)
	last := lines[len(lines)-1]
	assert.Equal(t, []string{"4400", `"ISP"`}, []string{last[7], last[36]})
	// Invoice 1 and its Storno have no position either: three documents
	// book on today's values.
	assert.Equal(t, 3, store.records[0].ResolvedPositions)
}

// A Sollstellung that maps where a row books writes the lines of each
// document; one that writes the account refuses a line without one.
func TestSollstellungWritesOneRowPerLineWhenItMapsTheAccounts(t *testing.T) {
	t.Parallel()
	mapping := parentbilling.SollstellungMapping{Columns: []parentbilling.SollstellungColumn{
		{Field: parentbilling.SollFieldInvoiceNumber, Header: "Nr"}, {Field: parentbilling.SollFieldAmount, Header: "Betrag"},
		{Field: parentbilling.SollFieldRevenueAccount, Header: "Konto"}, {Field: parentbilling.SollFieldCostCenter, Header: "KOST1"},
		{Field: parentbilling.SollFieldCostCenter2, Header: "KOST2"}},
		Delimiter: ";", DecimalSeparator: ",", DateFormat: parentbilling.SollDateGerman, Encoding: parentbilling.SollEncodingUTF8,
		HeaderRow: true, FileNamePattern: "soll-{NR}.csv"}
	invoices := []parentbilling.ExportInvoice{{Number: "MB-1", TotalCents: 11050, Parts: []parentbilling.ExportBookingPart{
		{AmountCents: 3250, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4410", CostCenter: "ISP", CostCenter2: "Frueh", Frozen: true}},
		{AmountCents: 7800, Accounts: parentbilling.PositionAccounts{RevenueAccount: "4400", CostCenter: "ISP", Frozen: true}}}}}
	rows, resolved, missing := sollRows(mapping, invoices, newBookingResolver(parentbilling.BookingAssignments{}, "4000"))
	require.Empty(t, missing)
	assert.Zero(t, resolved)
	data, err := renderSollRows(mapping, rows, 3)
	require.NoError(t, err)
	assert.Equal(t, "Nr;Betrag;Konto;KOST1;KOST2\r\nMB-1;32,50;4410;ISP;Frueh\r\nMB-1;78,00;4400;ISP;\r\n", string(data))

	// Without an account field the file keeps one row per document.
	mapping.Columns = mapping.Columns[:2]
	rows, _, _ = sollRows(mapping, invoices, newBookingResolver(parentbilling.BookingAssignments{}, "4000"))
	data, err = renderSollRows(mapping, rows, 3)
	require.NoError(t, err)
	assert.Equal(t, "Nr;Betrag\r\nMB-1;110,50\r\n", string(data))

	// No standard and a position without a revenue account: named.
	mapping.Columns = append(mapping.Columns, parentbilling.SollstellungColumn{Field: parentbilling.SollFieldRevenueAccount, Header: "Konto"})
	invoices[0].Parts[0].Accounts = parentbilling.PositionAccounts{}
	_, _, missing = sollRows(mapping, invoices, newBookingResolver(parentbilling.BookingAssignments{}, ""))
	assert.Equal(t, []string{"MB-1"}, missing)
	assert.True(t, strings.HasPrefix(parentbilling.SollFieldCostCenter2, "cost_center"))
}
