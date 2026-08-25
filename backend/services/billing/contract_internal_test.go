package billing

import (
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"

	// Blank import: the vertrag.* definitions register in defaults' init().
	// Without it the registry is empty and the drift check below is vacuous.
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func day(y int, m time.Month, d int) timezone.Date { return timezone.NewDate(y, m, d) }

func TestContractSettingKeys_IsACopy(t *testing.T) {
	t.Parallel()

	first := ContractSettingKeys()
	require.Len(t, first, 10)

	first[0] = "mutated"
	assert.Equal(t, configModel.KeyContractTier, ContractSettingKeys()[0],
		"callers must not be able to corrupt the package-level key list")
}

func TestTierLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Noch nicht hinterlegt", TierLabel(configModel.ContractTierUnset))
	assert.Equal(t, "Testphase", TierLabel(configModel.ContractTierTest))
	assert.Equal(t, "Basis", TierLabel(configModel.ContractTierBasis))
	assert.Equal(t, "Plus", TierLabel(configModel.ContractTierPlus))
	assert.Equal(t, "Premium", TierLabel(configModel.ContractTierPremium))
	assert.Equal(t, "enterprise", TierLabel("enterprise"),
		"an unknown stored value stays visible instead of rendering blank")
}

func TestBillingCycleLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Noch nicht hinterlegt", BillingCycleLabel(configModel.ContractCycleUnset))
	assert.Equal(t, "Monatlich", BillingCycleLabel(configModel.ContractCycleMonthly))
	assert.Equal(t, "Quartalsweise", BillingCycleLabel(configModel.ContractCycleQuarterly))
	assert.Equal(t, "Jährlich", BillingCycleLabel(configModel.ContractCycleYearly))
	assert.Equal(t, "woechentlich", BillingCycleLabel("woechentlich"))
}

// TestLabelsCoverRegistryOptions keeps the label maps and the registry select
// options from drifting apart — a stored value without a label would render as
// a raw token in the school's UI.
func TestLabelsCoverRegistryOptions(t *testing.T) {
	t.Parallel()

	tier := configModel.GetDefinition(configModel.KeyContractTier)
	require.NotNil(t, tier)
	require.NotNil(t, tier.Options)
	for _, option := range tier.Options.Static {
		value, ok := option.Value.(string)
		require.True(t, ok)
		assert.Containsf(t, tierLabels, value, "tier %q has no label", value)
	}

	cycle := configModel.GetDefinition(configModel.KeyContractBillingCycle)
	require.NotNil(t, cycle)
	require.NotNil(t, cycle.Options)
	for _, option := range cycle.Options.Static {
		value, ok := option.Value.(string)
		require.True(t, ok)
		assert.Containsf(t, cycleLabels, value, "cycle %q has no label", value)
	}
}

func TestNormalizeInput_TrimsFreeText(t *testing.T) {
	t.Parallel()

	got := normalizeInput(InvoiceInput{
		PeriodLabel:   "  Januar 2026  ",
		InvoiceNumber: "  R-1  ",
		Note:          "  bitte bis Monatsende  ",
		Status:        platform.InvoiceStatusOpen,
	}, day(2026, time.January, 15))

	assert.Equal(t, "Januar 2026", got.PeriodLabel)
	assert.Equal(t, "R-1", got.InvoiceNumber)
	assert.Equal(t, "bitte bis Monatsende", got.Note)
}

func TestNormalizeInput_DefaultsStatusToOpen(t *testing.T) {
	t.Parallel()

	got := normalizeInput(InvoiceInput{PeriodLabel: "Januar"}, day(2026, time.January, 15))

	assert.Equal(t, platform.InvoiceStatusOpen, got.Status)
	assert.Nil(t, got.PaidOn)
}

// Marking an invoice paid without naming a day means "paid today". A business
// convenience, therefore in the service and not in the model.
func TestNormalizeInput_PaidWithoutDateMeansToday(t *testing.T) {
	t.Parallel()

	today := day(2026, time.February, 3)
	got := normalizeInput(InvoiceInput{
		PeriodLabel: "Januar",
		Status:      platform.InvoiceStatusPaid,
	}, today)

	require.NotNil(t, got.PaidOn)
	assert.Equal(t, today, *got.PaidOn)
}

func TestNormalizeInput_PaidKeepsExplicitDate(t *testing.T) {
	t.Parallel()

	explicit := day(2026, time.January, 28)
	got := normalizeInput(InvoiceInput{
		PeriodLabel: "Januar",
		Status:      platform.InvoiceStatusPaid,
		PaidOn:      &explicit,
	}, day(2026, time.February, 3))

	require.NotNil(t, got.PaidOn)
	assert.Equal(t, explicit, *got.PaidOn)
}

// Moving an invoice away from "bezahlt" drops the payment date instead of
// failing — otherwise correcting a wrongly-marked payment would be impossible
// without two round trips.
func TestNormalizeInput_NonPaidClearsPaymentDate(t *testing.T) {
	t.Parallel()

	paid := day(2026, time.January, 28)

	for _, status := range []string{platform.InvoiceStatusOpen, platform.InvoiceStatusCancelled} {
		got := normalizeInput(InvoiceInput{
			PeriodLabel: "Januar",
			Status:      status,
			PaidOn:      &paid,
		}, day(2026, time.February, 3))

		assert.Nilf(t, got.PaidOn, "status %q must not keep a payment date", status)
	}
}

func TestApplyInput_CopiesEveryEditableField(t *testing.T) {
	t.Parallel()

	paid := day(2026, time.January, 28)
	invoice := &platform.SchoolInvoice{}
	invoice.SetTenantID(7)

	applyInput(invoice, InvoiceInput{
		PeriodLabel:   "Januar 2026",
		InvoiceNumber: "R-1",
		AmountCents:   12345,
		DueDate:       day(2026, time.January, 31),
		Status:        platform.InvoiceStatusPaid,
		PaidOn:        &paid,
		Note:          "Hinweis",
	})

	assert.Equal(t, "Januar 2026", invoice.PeriodLabel)
	assert.Equal(t, "R-1", invoice.InvoiceNumber)
	assert.Equal(t, int64(12345), invoice.AmountCents)
	assert.Equal(t, day(2026, time.January, 31), invoice.DueDate)
	assert.Equal(t, platform.InvoiceStatusPaid, invoice.Status)
	require.NotNil(t, invoice.PaidOn)
	assert.Equal(t, paid, *invoice.PaidOn)
	assert.Equal(t, "Hinweis", invoice.Note)
	assert.Equal(t, int64(7), invoice.GetTenantID(), "applyInput must never touch the tenant")
}

func TestToViews_DerivesOverdueAndSkipsNils(t *testing.T) {
	t.Parallel()

	today := day(2026, time.February, 1)
	overdue := &platform.SchoolInvoice{
		PeriodLabel: "Januar",
		AmountCents: 100,
		DueDate:     day(2026, time.January, 31),
		Status:      platform.InvoiceStatusOpen,
	}
	overdue.ID = 1
	future := &platform.SchoolInvoice{
		PeriodLabel: "Februar",
		AmountCents: 200,
		DueDate:     day(2026, time.February, 28),
		Status:      platform.InvoiceStatusOpen,
	}
	future.ID = 2

	views := ToViews([]*platform.SchoolInvoice{overdue, nil, future}, today)

	require.Len(t, views, 2, "nil rows are skipped, not rendered as empty invoices")
	assert.True(t, views[0].Overdue)
	assert.Equal(t, int64(1), views[0].ID)
	assert.False(t, views[1].Overdue)
}

func TestToViews_EmptyInputYieldsEmptySlice(t *testing.T) {
	t.Parallel()

	views := ToViews(nil, day(2026, time.February, 1))

	assert.NotNil(t, views, "a nil slice would serialise as JSON null, not []")
	assert.Empty(t, views)
}

// summarizeInvoices receives due-date DESC input (the repository contract).
func TestSummarizeInvoices_SumsOnlyOpenInvoices(t *testing.T) {
	t.Parallel()

	views := []InvoiceView{
		{ID: 3, AmountCents: 300, Status: platform.InvoiceStatusOpen, DueDate: day(2026, time.March, 31)},
		{ID: 2, AmountCents: 200, Status: platform.InvoiceStatusCancelled, DueDate: day(2026, time.February, 28)},
		{ID: 1, AmountCents: 100, Status: platform.InvoiceStatusPaid, DueDate: day(2026, time.January, 31)},
	}

	open, next := summarizeInvoices(views)

	assert.Equal(t, int64(300), open, "paid and cancelled invoices owe nothing")
	require.NotNil(t, next)
	assert.Equal(t, int64(3), next.ID)
}

func TestSummarizeInvoices_PrefersTheOldestOverdueInvoice(t *testing.T) {
	t.Parallel()

	views := []InvoiceView{
		{ID: 4, AmountCents: 400, Status: platform.InvoiceStatusOpen, DueDate: day(2026, time.April, 30)},
		{ID: 3, AmountCents: 300, Status: platform.InvoiceStatusOpen, DueDate: day(2026, time.March, 31), Overdue: true},
		{ID: 2, AmountCents: 200, Status: platform.InvoiceStatusOpen, DueDate: day(2026, time.February, 28), Overdue: true},
	}

	open, next := summarizeInvoices(views)

	assert.Equal(t, int64(900), open)
	require.NotNil(t, next)
	assert.Equal(t, int64(2), next.ID,
		"a school that is already late must see the oldest missed date first")
}

func TestSummarizeInvoices_NothingOpen(t *testing.T) {
	t.Parallel()

	open, next := summarizeInvoices([]InvoiceView{
		{ID: 1, AmountCents: 100, Status: platform.InvoiceStatusPaid},
	})

	assert.Zero(t, open)
	assert.Nil(t, next)
}

func TestSummarizeInvoices_Empty(t *testing.T) {
	t.Parallel()

	open, next := summarizeInvoices(nil)

	assert.Zero(t, open)
	assert.Nil(t, next)
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	termStart := day(2026, time.January, 1)

	tests := []struct {
		name     string
		overview ContractOverview
		want     bool
	}{
		{name: "nothing entered", overview: ContractOverview{}, want: false},
		{name: "tier set", overview: ContractOverview{Tier: configModel.ContractTierBasis}, want: true},
		{name: "contingent set", overview: ContractOverview{BookedChildren: 120}, want: true},
		{name: "price set", overview: ContractOverview{PricePerChildCents: 100}, want: true},
		{name: "cycle set", overview: ContractOverview{BillingCycle: configModel.ContractCycleMonthly}, want: true},
		{name: "term start set", overview: ContractOverview{TermStart: &termStart}, want: true},
		{name: "term end set", overview: ContractOverview{TermEnd: &termStart}, want: true},
		{name: "recipient set", overview: ContractOverview{InvoiceRecipient: "a@b.de"}, want: true},
		{name: "customer number set", overview: ContractOverview{CustomerNumber: "K-1"}, want: true},
		{
			// An invoice without contract facts still means "there is a
			// contract" — claiming otherwise above a list of bills would be
			// the exact contradiction the Verständlichkeit rule forbids.
			name:     "only invoices",
			overview: ContractOverview{Invoices: []InvoiceView{{ID: 1}}},
			want:     true,
		},
		{
			name:     "support email alone is not a contract",
			overview: ContractOverview{SupportEmail: "hilfe@moto.nrw"},
			want:     false,
		},
		{
			name:     "a note alone is not a contract",
			overview: ContractOverview{Note: "Bitte melden"},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			overview := tc.overview
			assert.Equal(t, tc.want, isConfigured(&overview))
		})
	}
}

func TestWrapWriteError(t *testing.T) {
	t.Parallel()

	assert.NoError(t, wrapWriteError("create", nil))

	unique := errors.New(`ERROR: duplicate key value violates unique constraint "uq_school_invoices_number"`)
	assert.ErrorIs(t, wrapWriteError("create", unique), ErrInvoiceNumberTaken)

	other := errors.New("connection refused")
	wrapped := wrapWriteError("create invoice", other)
	assert.ErrorIs(t, wrapped, other)
	assert.Contains(t, wrapped.Error(), "create invoice")
}
