package billing

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test doubles -----------------------------------------------------------
//
// Hand-rolled rather than shared (Rule 13): these are error-injection doubles
// with per-call hooks, which is the documented exception. No shared mock for
// platform.SchoolInvoiceRepository exists.

type fakeInvoiceRepo struct {
	rows    []*platform.SchoolInvoice
	nextID  int64
	listErr error
	findErr error
	saveErr error
	delErr  error

	created []*platform.SchoolInvoice
	updated []*platform.SchoolInvoice
	deleted []int64
}

func newFakeInvoiceRepo(rows ...*platform.SchoolInvoice) *fakeInvoiceRepo {
	return &fakeInvoiceRepo{rows: rows, nextID: 100}
}

func (f *fakeInvoiceRepo) Create(_ context.Context, entity *platform.SchoolInvoice) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.nextID++
	entity.ID = f.nextID
	f.created = append(f.created, entity)
	f.rows = append(f.rows, entity)
	return nil
}

func (f *fakeInvoiceRepo) FindByID(_ context.Context, id any) (*platform.SchoolInvoice, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	for _, row := range f.rows {
		if row.ID == id.(int64) {
			return row, nil
		}
	}
	return nil, errors.New("no rows")
}

func (f *fakeInvoiceRepo) FindByIDOrNil(_ context.Context, id any) (*platform.SchoolInvoice, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	for _, row := range f.rows {
		if row.ID == id.(int64) {
			return row, nil
		}
	}
	return nil, nil
}

func (f *fakeInvoiceRepo) Update(_ context.Context, entity *platform.SchoolInvoice) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.updated = append(f.updated, entity)
	return nil
}

func (f *fakeInvoiceRepo) Delete(_ context.Context, id any) error {
	if f.delErr != nil {
		return f.delErr
	}
	f.deleted = append(f.deleted, id.(int64))
	return nil
}

func (f *fakeInvoiceRepo) List(_ context.Context, _ map[string]any) ([]*platform.SchoolInvoice, error) {
	return f.rows, nil
}

func (f *fakeInvoiceRepo) ListForTenant(_ context.Context) ([]*platform.SchoolInvoice, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rows, nil
}

type fakeSettings struct {
	strings map[string]string
	ints    map[string]int
	err     error
}

func (f *fakeSettings) ResolveString(_ context.Context, key string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.strings[key], nil
}

func (f *fakeSettings) ResolveInt(_ context.Context, key string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.ints[key], nil
}

type fakeStudents struct {
	count   int
	err     error
	gotOpts *modelBase.QueryOptions
}

func (f *fakeStudents) CountWithOptions(_ context.Context, options *modelBase.QueryOptions) (int, error) {
	f.gotOpts = options
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

// --- helpers ----------------------------------------------------------------

const testTenantID int64 = 4711

func fixedToday() timezone.Date { return timezone.NewDate(2026, time.February, 15) }

func newTestService(repo platform.SchoolInvoiceRepository, settings settingsResolver, students studentCounter) Service {
	return NewService(Config{
		Invoices: repo,
		Students: students,
		Settings: settings,
		// No DB: inTenantTx then runs fn against a tenant-tagged context.
		Logger: slog.New(slog.DiscardHandler),
		Now:    fixedToday,
	})
}

func tenantCtx() context.Context {
	return tenant.WithTenantID(context.Background(), testTenantID)
}

func invoice(id int64, label string, cents int64, due timezone.Date, status string) *platform.SchoolInvoice {
	row := &platform.SchoolInvoice{
		PeriodLabel: label,
		AmountCents: cents,
		DueDate:     due,
		Status:      status,
	}
	row.ID = id
	row.SetTenantID(testTenantID)
	return row
}

// --- NewService -------------------------------------------------------------

func TestNewService_DefaultsLoggerAndClock(t *testing.T) {
	t.Parallel()

	svc, ok := NewService(Config{Invoices: newFakeInvoiceRepo()}).(*service)
	require.True(t, ok)

	assert.NotNil(t, svc.logger, "a nil logger would panic on the first warning")
	require.NotNil(t, svc.now)
	assert.Equal(t, timezone.TodayDate(), svc.now())
}

// --- GetOverview ------------------------------------------------------------

func TestGetOverview_RequiresTenantContext(t *testing.T) {
	t.Parallel()

	svc := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, &fakeStudents{})

	_, err := svc.GetOverview(context.Background())

	assert.ErrorIs(t, err, ErrNoTenantContext,
		"without a tenant the query would silently read nothing and look like an empty contract")
}

func TestGetOverview_BuildsFullPayload(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo(
		invoice(3, "März 2026", 30000, day(2026, time.March, 31), platform.InvoiceStatusOpen),
		invoice(2, "Februar 2026", 20000, day(2026, time.February, 10), platform.InvoiceStatusOpen),
		invoice(1, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusPaid),
	)
	settings := &fakeSettings{
		strings: map[string]string{
			configModel.KeyContractTier:             configModel.ContractTierPlus,
			configModel.KeyContractBillingCycle:     configModel.ContractCycleMonthly,
			configModel.KeyContractTermStart:        "2026-01-01",
			configModel.KeyContractTermEnd:          "2026-12-31",
			configModel.KeyContractInvoiceRecipient: "buchhaltung@schule.de",
			configModel.KeyContractCustomerNumber:   "K-10023",
			configModel.KeyContractSupportEmail:     "rechnung@moto.nrw",
			configModel.KeyContractNote:             "Preis gilt bis Schuljahresende.",
		},
		ints: map[string]int{
			configModel.KeyContractBookedChildren:     150,
			configModel.KeyContractPricePerChildCents: 200,
		},
	}
	students := &fakeStudents{count: 163}

	overview, err := newTestService(repo, settings, students).GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.Equal(t, configModel.ContractTierPlus, overview.Tier)
	assert.Equal(t, "Plus", overview.TierLabel)
	assert.Equal(t, "Monatlich", overview.BillingCycleLabel)
	assert.Equal(t, 150, overview.BookedChildren)
	assert.Equal(t, 163, overview.ActiveChildren)
	assert.Equal(t, 200, overview.PricePerChildCents)
	require.NotNil(t, overview.TermStart)
	assert.Equal(t, "2026-01-01", overview.TermStart.String())
	require.NotNil(t, overview.TermEnd)
	assert.Equal(t, "2026-12-31", overview.TermEnd.String())
	assert.Equal(t, "buchhaltung@schule.de", overview.InvoiceRecipient)
	assert.Equal(t, "K-10023", overview.CustomerNumber)
	assert.Equal(t, "rechnung@moto.nrw", overview.SupportEmail)
	assert.Equal(t, "Preis gilt bis Schuljahresende.", overview.Note)
	assert.True(t, overview.Configured)
	assert.Equal(t, fixedToday(), overview.ReferenceDate)

	require.Len(t, overview.Invoices, 3)
	assert.Equal(t, int64(50000), overview.OpenAmountCents, "only the two open invoices count")
	require.NotNil(t, overview.NextDue)
	assert.Equal(t, int64(2), overview.NextDue.ID)
	assert.True(t, overview.NextDue.Overdue, "10 February is behind the 15 February reference day")
}

func TestGetOverview_UnconfiguredSchool(t *testing.T) {
	t.Parallel()

	overview, err := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, &fakeStudents{}).
		GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.False(t, overview.Configured)
	assert.Empty(t, overview.Invoices)
	assert.Zero(t, overview.OpenAmountCents)
	assert.Nil(t, overview.NextDue)
	assert.Equal(t, "Noch nicht hinterlegt", overview.TierLabel)
}

// The invoice list is the reason the school opened the page. A failure there
// must not be papered over with an empty schedule that reads as "nothing owed".
func TestGetOverview_InvoiceReadFailureIsFatal(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()
	repo.listErr = errors.New("connection reset")

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).GetOverview(tenantCtx())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection reset")
}

// Settings and the child count degrade instead: the payment schedule stays
// readable even if the registry read fails.
func TestGetOverview_SettingsFailureDegradesGracefully(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo(
		invoice(1, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen),
	)
	settings := &fakeSettings{err: errors.New("settings unavailable")}

	overview, err := newTestService(repo, settings, &fakeStudents{count: 5}).GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.Equal(t, configModel.ContractTierUnset, overview.Tier)
	assert.Zero(t, overview.BookedChildren)
	assert.Len(t, overview.Invoices, 1, "the schedule survives a settings outage")
	assert.True(t, overview.Configured, "the invoice alone still means there is a contract")
}

func TestGetOverview_ChildCountFailureYieldsZero(t *testing.T) {
	t.Parallel()

	students := &fakeStudents{err: errors.New("boom")}

	overview, err := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, students).GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.Zero(t, overview.ActiveChildren)
}

func TestGetOverview_WithoutCollaboratorsStillWorks(t *testing.T) {
	t.Parallel()

	svc := NewService(Config{
		Invoices: newFakeInvoiceRepo(),
		Logger:   slog.New(slog.DiscardHandler),
		Now:      fixedToday,
	})

	overview, err := svc.GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.Zero(t, overview.ActiveChildren)
	assert.False(t, overview.Configured)
}

// The contingent compares against children in care today. Counting pending or
// graduated children would make the number impossible to reconcile with the
// Kinderliste the school sees elsewhere.
func TestGetOverview_CountsOnlyActiveChildren(t *testing.T) {
	t.Parallel()

	students := &fakeStudents{count: 12}

	_, err := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, students).GetOverview(tenantCtx())
	require.NoError(t, err)

	require.NotNil(t, students.gotOpts)
	require.NotNil(t, students.gotOpts.Filter)
	value, ok := students.gotOpts.Filter.Get("status")
	require.True(t, ok, "the count must be filtered by lifecycle status")
	assert.Equal(t, "active", value)
}

func TestGetOverview_NegativeSettingValuesAreClamped(t *testing.T) {
	t.Parallel()

	settings := &fakeSettings{ints: map[string]int{
		configModel.KeyContractBookedChildren:     -5,
		configModel.KeyContractPricePerChildCents: -1,
	}}

	overview, err := newTestService(newFakeInvoiceRepo(), settings, &fakeStudents{}).GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.Zero(t, overview.BookedChildren)
	assert.Zero(t, overview.PricePerChildCents)
}

// A stored value that is not a calendar date is treated as unset: showing a
// wrong contract date is worse than showing none.
func TestGetOverview_UnparseableTermDateIsTreatedAsUnset(t *testing.T) {
	t.Parallel()

	settings := &fakeSettings{strings: map[string]string{
		configModel.KeyContractTermStart: "01.01.2026",
	}}

	overview, err := newTestService(newFakeInvoiceRepo(), settings, &fakeStudents{}).GetOverview(tenantCtx())
	require.NoError(t, err)

	assert.Nil(t, overview.TermStart)
}

// --- operator writes --------------------------------------------------------

func TestListInvoices_RejectsMissingTenant(t *testing.T) {
	t.Parallel()

	svc := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, &fakeStudents{})

	_, err := svc.ListInvoices(context.Background(), 0)

	assert.ErrorIs(t, err, ErrNoTenantContext)
}

func TestListInvoices_ReturnsViewsWithDerivedOverdue(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo(
		invoice(1, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen),
	)

	views, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).
		ListInvoices(context.Background(), testTenantID)
	require.NoError(t, err)

	require.Len(t, views, 1)
	assert.True(t, views[0].Overdue)
}

func TestListInvoices_PropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()
	repo.listErr = errors.New("db down")

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).
		ListInvoices(context.Background(), testTenantID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestCreateInvoice_StampsTenantAndReturnsView(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()

	view, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).CreateInvoice(
		context.Background(), testTenantID, InvoiceInput{
			PeriodLabel: "  Februar 2026 ",
			AmountCents: 19900,
			DueDate:     day(2026, time.February, 28),
		})
	require.NoError(t, err)

	require.Len(t, repo.created, 1)
	assert.Equal(t, testTenantID, repo.created[0].GetTenantID())
	assert.Equal(t, "Februar 2026", repo.created[0].PeriodLabel)
	assert.Equal(t, platform.InvoiceStatusOpen, repo.created[0].Status)

	require.NotNil(t, view)
	assert.Equal(t, repo.created[0].ID, view.ID)
	assert.False(t, view.Overdue, "28 February is after the 15 February reference day")
}

func TestCreateInvoice_ValidatesBeforeTouchingTheDatabase(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).CreateInvoice(
		context.Background(), testTenantID, InvoiceInput{
			PeriodLabel: "",
			DueDate:     day(2026, time.February, 28),
		})

	assert.ErrorIs(t, err, platform.ErrInvoicePeriodLabelRequired)
	assert.Empty(t, repo.created)
}

func TestCreateInvoice_MapsDuplicateNumber(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()
	repo.saveErr = errors.New(`duplicate key value violates unique constraint "uq_school_invoices_number"`)

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).CreateInvoice(
		context.Background(), testTenantID, InvoiceInput{
			PeriodLabel:   "Februar 2026",
			InvoiceNumber: "R-1",
			DueDate:       day(2026, time.February, 28),
		})

	assert.ErrorIs(t, err, ErrInvoiceNumberTaken)
}

func TestCreateInvoice_RejectsMissingTenant(t *testing.T) {
	t.Parallel()

	_, err := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, &fakeStudents{}).CreateInvoice(
		context.Background(), 0, InvoiceInput{
			PeriodLabel: "Februar 2026",
			DueDate:     day(2026, time.February, 28),
		})

	assert.ErrorIs(t, err, ErrNoTenantContext)
}

func TestUpdateInvoice_MarksPaidWithTodayWhenNoDateGiven(t *testing.T) {
	t.Parallel()

	row := invoice(7, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen)
	repo := newFakeInvoiceRepo(row)

	view, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).UpdateInvoice(
		context.Background(), testTenantID, 7, InvoiceInput{
			PeriodLabel: "Januar 2026",
			AmountCents: 10000,
			DueDate:     day(2026, time.January, 31),
			Status:      platform.InvoiceStatusPaid,
		})
	require.NoError(t, err)

	require.Len(t, repo.updated, 1)
	require.NotNil(t, repo.updated[0].PaidOn)
	assert.Equal(t, fixedToday(), *repo.updated[0].PaidOn)

	require.NotNil(t, view)
	assert.Equal(t, platform.InvoiceStatusPaid, view.Status)
	assert.False(t, view.Overdue, "a paid invoice is never overdue, however late it was")
}

func TestUpdateInvoice_UnknownIDIsNotFound(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).UpdateInvoice(
		context.Background(), testTenantID, 999, InvoiceInput{
			PeriodLabel: "Januar",
			DueDate:     day(2026, time.January, 31),
		})

	assert.ErrorIs(t, err, ErrInvoiceNotFound)
	assert.Empty(t, repo.updated)
}

// A real read failure must not be reported as "not found": that would tell the
// operator the invoice is gone when the database is merely unreachable.
func TestUpdateInvoice_ReadFailureIsNotReportedAsNotFound(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()
	repo.findErr = errors.New("connection reset")

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).UpdateInvoice(
		context.Background(), testTenantID, 1, InvoiceInput{
			PeriodLabel: "Januar",
			DueDate:     day(2026, time.January, 31),
		})

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvoiceNotFound)
	assert.Contains(t, err.Error(), "connection reset")
}

func TestUpdateInvoice_RejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	row := invoice(7, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen)
	repo := newFakeInvoiceRepo(row)

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).UpdateInvoice(
		context.Background(), testTenantID, 7, InvoiceInput{
			PeriodLabel: "Januar 2026",
			AmountCents: -1,
			DueDate:     day(2026, time.January, 31),
		})

	assert.ErrorIs(t, err, platform.ErrInvoiceAmountNegative)
	assert.Empty(t, repo.updated)
}

func TestUpdateInvoice_MapsWriteFailure(t *testing.T) {
	t.Parallel()

	row := invoice(7, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen)
	repo := newFakeInvoiceRepo(row)
	repo.saveErr = errors.New("write timeout")

	_, err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).UpdateInvoice(
		context.Background(), testTenantID, 7, InvoiceInput{
			PeriodLabel: "Januar 2026",
			DueDate:     day(2026, time.January, 31),
		})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "write timeout")
}

func TestDeleteInvoice_RemovesExistingRow(t *testing.T) {
	t.Parallel()

	row := invoice(7, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen)
	repo := newFakeInvoiceRepo(row)

	require.NoError(t, newTestService(repo, &fakeSettings{}, &fakeStudents{}).
		DeleteInvoice(context.Background(), testTenantID, 7))

	assert.Equal(t, []int64{7}, repo.deleted)
}

func TestDeleteInvoice_UnknownIDIsNotFound(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()

	err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).
		DeleteInvoice(context.Background(), testTenantID, 42)

	assert.ErrorIs(t, err, ErrInvoiceNotFound)
	assert.Empty(t, repo.deleted)
}

func TestDeleteInvoice_ReadFailureIsNotReportedAsNotFound(t *testing.T) {
	t.Parallel()

	repo := newFakeInvoiceRepo()
	repo.findErr = errors.New("connection reset")

	err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).
		DeleteInvoice(context.Background(), testTenantID, 1)

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvoiceNotFound)
}

func TestDeleteInvoice_PropagatesDeleteFailure(t *testing.T) {
	t.Parallel()

	row := invoice(7, "Januar 2026", 10000, day(2026, time.January, 31), platform.InvoiceStatusOpen)
	repo := newFakeInvoiceRepo(row)
	repo.delErr = errors.New("deadlock detected")

	err := newTestService(repo, &fakeSettings{}, &fakeStudents{}).
		DeleteInvoice(context.Background(), testTenantID, 7)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadlock detected")
}

func TestDeleteInvoice_RejectsMissingTenant(t *testing.T) {
	t.Parallel()

	err := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, &fakeStudents{}).
		DeleteInvoice(context.Background(), 0, 1)

	assert.ErrorIs(t, err, ErrNoTenantContext)
}

// inTenantTx must put the tenant into the context even without a *bun.DB, so
// the repository layer's defense-in-depth tenant filter still applies.
func TestInTenantTx_TagsContextWithoutDB(t *testing.T) {
	t.Parallel()

	svc, ok := newTestService(newFakeInvoiceRepo(), &fakeSettings{}, &fakeStudents{}).(*service)
	require.True(t, ok)

	var seen int64
	require.NoError(t, svc.inTenantTx(context.Background(), testTenantID, func(ctx context.Context) error {
		seen = tenant.FromContext(ctx)
		return nil
	}))

	assert.Equal(t, testTenantID, seen)
}
