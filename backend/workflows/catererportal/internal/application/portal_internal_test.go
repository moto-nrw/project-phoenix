package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

// steps records the order in which the portal locks and calls the owners.
type steps []string

func (s *steps) add(step string) {
	if s != nil {
		*s = append(*s, step)
	}
}

type fakeCaterers struct {
	steps       *steps
	caterer     identityaccess.Caterer
	supply      identityaccess.CatererSupply
	active      bool
	schools     []int64
	invitations []identityaccess.CatererInvitation
	acceptance  identityaccess.CatererAcceptance
	endedBy     int64
	err         error
}

func (f *fakeCaterers) CreateCatererInvitation(context.Context, identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error) {
	f.steps.add("identity:invite")
	return identityaccess.CatererInvitation{}, f.err
}
func (f *fakeCaterers) ListCatererInvitations(context.Context) ([]identityaccess.CatererInvitation, error) {
	return f.invitations, f.err
}
func (f *fakeCaterers) RevokeCatererInvitation(context.Context, int64) error { return f.err }
func (f *fakeCaterers) PreviewCatererInvitation(context.Context, string) (identityaccess.CatererInvitationPreview, error) {
	return identityaccess.CatererInvitationPreview{}, f.err
}
func (f *fakeCaterers) AcceptCatererInvitation(context.Context, string, identityaccess.CatererRegistration) (identityaccess.CatererAcceptance, error) {
	f.steps.add("identity:accept")
	return f.acceptance, f.err
}
func (f *fakeCaterers) EndCatererSchool(_ context.Context, accountID int64) (bool, error) {
	f.steps.add("identity:end")
	f.endedBy = accountID
	return f.active, f.err
}
func (f *fakeCaterers) SchoolCaterer(context.Context) (identityaccess.CatererSupply, bool, error) {
	f.steps.add("identity:supply")
	return f.supply, f.active, f.err
}
func (f *fakeCaterers) CatererSchools(context.Context, int64) (identityaccess.Caterer, []int64, error) {
	return f.caterer, f.schools, f.err
}
func (f *fakeCaterers) LoginCatererWithAudit(context.Context, string, string, string, string) (string, string, error) {
	return "access", "refresh", f.err
}

type fakeOrders struct {
	steps       *steps
	since       time.Time
	ordersRead  bool
	confirmedBy int64
	week        mealplan.OrderWeek
	err         error
}

func (f *fakeOrders) CatererOrders(_ context.Context, since time.Time, _, _ mealplan.Date) (mealplan.OrderWeek, error) {
	f.steps.add("mealplan:read")
	f.since = since
	if f.week.Days != nil {
		return f.week, f.err
	}
	return mealplan.OrderWeek{Rhythm: "daily"}, f.err
}
func (f *fakeOrders) Orders(context.Context, mealplan.Date, mealplan.Date) (mealplan.OrderWeek, error) {
	f.ordersRead = true
	return f.week, f.err
}
func (f *fakeOrders) ConfirmOrder(_ context.Context, _ int64, since time.Time, accountID int64) error {
	f.steps.add("mealplan:confirm")
	f.since = since
	f.confirmedBy = accountID
	return f.err
}

// schoolsOpened records each school transaction the portal opens.
type schoolsOpened []int64

func (s *schoolsOpened) within(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	*s = append(*s, schoolID)
	return fn(ctx)
}

func newTestPortal(t *testing.T, caterers *fakeCaterers, orders *fakeOrders, opened *schoolsOpened) *Portal {
	t.Helper()
	portal, err := New(Dependencies{
		Caterers: caterers, Orders: orders, WithinSchool: opened.within,
		CurrentSchool: func(context.Context) int64 { return 11 },
		Lock: func(_ context.Context, key string) error {
			caterers.steps.add("lock:" + key)
			return nil
		},
		SchoolNames: func(_ context.Context, ids []int64) (map[int64]string, error) {
			names := map[int64]string{}
			for _, id := range ids {
				names[id] = "Schule"
			}
			return names, nil
		},
	})
	require.NoError(t, err)
	return portal
}

var supplyStart = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

func supplying(catererID int64) identityaccess.CatererSupply {
	return identityaccess.CatererSupply{Caterer: identityaccess.Caterer{ID: catererID, Name: "Küche"}, DeliveryAddress: "Hof", StartedAt: supplyStart}
}

// ADR 0077: the caterer reads and confirms only in a school it supplies, and
// each school in its own transaction.
func TestPortalReadsOnlySuppliedSchools(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7, Name: "Küche"}, schools: []int64{11, 12}, supply: supplying(7), active: true}
	orders := &fakeOrders{}
	var opened schoolsOpened
	portal := newTestPortal(t, caterers, orders, &opened)

	_, err := portal.SchoolOrders(context.Background(), 3, 99, "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, catererportal.ErrSchoolNotSupplied)
	assert.Empty(t, opened, "a foreign school is never opened")
	require.ErrorIs(t, portal.ConfirmOrder(context.Background(), 3, 99, 5), catererportal.ErrSchoolNotSupplied)

	result, err := portal.SchoolOrders(context.Background(), 3, 12, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Equal(t, schoolsOpened{12}, opened)
	assert.Equal(t, supplyStart, orders.since, "the Meal Plan reads from the start of the supply on")
	assert.Equal(t, "Hof", result.DeliveryAddress)
	require.NoError(t, portal.ConfirmOrder(context.Background(), 3, 11, 5))
	assert.Equal(t, int64(3), orders.confirmedBy, "the caterer's account confirms")

	schools, err := portal.Schools(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, catererportal.CatererSchools{CatererName: "Küche", Schools: []catererportal.School{{ID: 11, Name: "Schule"}, {ID: 12, Name: "Schule"}}}, schools)
}

// ADR 0077: the school's transaction checks the supply again; a caterer
// whose supply ended there reads and confirms nothing.
func TestPortalChecksTheSupplyInTheSchoolTransaction(t *testing.T) {
	t.Parallel()
	for name, supply := range map[string]*fakeCaterers{
		"ended":   {caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11}},
		"another": {caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11}, supply: supplying(8), active: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			orders := &fakeOrders{}
			var opened schoolsOpened
			portal := newTestPortal(t, supply, orders, &opened)
			_, err := portal.SchoolOrders(context.Background(), 3, 11, "2026-09-07", "2026-09-11")
			require.ErrorIs(t, err, catererportal.ErrSchoolNotSupplied)
			require.ErrorIs(t, portal.ConfirmOrder(context.Background(), 3, 11, 5), catererportal.ErrSchoolNotSupplied)
			assert.Zero(t, orders.confirmedBy)
			assert.False(t, orders.since.After(time.Time{}), "the Meal Plan is not asked")
		})
	}
}

// ADR 0067 addendum, condition 3: the workflow owns the supply lock. It is
// taken in the school's transaction before Identity & Access reads or
// writes the supply and before the Meal Plan confirms against it; the owners
// take none themselves.
func TestPortalLocksTheSupplyBeforeEitherOwner(t *testing.T) {
	t.Parallel()
	lock := fmt.Sprintf("lock:caterer-school:%d", 11)
	for name, tc := range map[string]struct {
		run  func(*Portal) error
		want steps
	}{
		"invite": {func(p *Portal) error {
			_, err := p.CreateCatererInvitation(context.Background(), identityaccess.CatererInvitationRequest{})
			return err
		}, steps{lock, "identity:invite"}},
		"accept": {func(p *Portal) error {
			return p.AcceptInvitation(context.Background(), 11, "token", identityaccess.CatererRegistration{})
		}, steps{lock, "identity:accept"}},
		"end": {func(p *Portal) error {
			_, err := p.EndSupply(context.Background(), 4)
			return err
		}, steps{lock, "identity:end"}},
		"confirm": {func(p *Portal) error {
			return p.ConfirmOrder(context.Background(), 3, 11, 5)
		}, steps{lock, "identity:supply", "mealplan:confirm"}},
		"read without lock": {func(p *Portal) error {
			_, err := p.SchoolOrders(context.Background(), 3, 11, "2026-09-07", "2026-09-11")
			return err
		}, steps{"identity:supply", "mealplan:read"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var got steps
			caterers := &fakeCaterers{steps: &got, caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11}, supply: supplying(7), active: true}
			var opened schoolsOpened
			require.NoError(t, tc.run(newTestPortal(t, caterers, &fakeOrders{steps: &got}, &opened)))
			assert.Equal(t, tc.want, got)
		})
	}
}

// ADR 0077: the acceptance runs in the school the link names; Identity &
// Access starts the supply with it, so no second owner writes.
func TestPortalAcceptanceRunsInTheLinkedSchool(t *testing.T) {
	t.Parallel()
	var got steps
	caterers := &fakeCaterers{steps: &got, acceptance: identityaccess.CatererAcceptance{AccountID: 3, CatererID: 7, TenantID: 11}}
	var opened schoolsOpened
	require.NoError(t, newTestPortal(t, caterers, &fakeOrders{steps: &got}, &opened).AcceptInvitation(context.Background(), 11, "token", identityaccess.CatererRegistration{}))
	assert.Equal(t, schoolsOpened{11}, opened)
	assert.Equal(t, steps{"lock:caterer-school:11", "identity:accept"}, got)
}

// ADR 0077: ending the supply records who ended it; a failure leaves the
// caller's transaction to roll back.
func TestPortalEndsTheSupply(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{active: true}
	var opened schoolsOpened
	ended, err := newTestPortal(t, caterers, &fakeOrders{}, &opened).EndSupply(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, ended)
	assert.Equal(t, int64(1), caterers.endedBy)

	_, err = newTestPortal(t, &fakeCaterers{err: errors.New("database down")}, &fakeOrders{}, &opened).EndSupply(context.Background(), 1)
	require.Error(t, err)
}

// The school's page names the caterer of the running supply, not an
// invitation the retention may have deleted.
func TestSchoolCatererNamesTheSupplyingCaterer(t *testing.T) {
	t.Parallel()
	supply := supplying(7)
	supply.Caterer.Name = "Küche am Markt"
	var opened schoolsOpened
	page, err := newTestPortal(t, &fakeCaterers{supply: supply, active: true}, &fakeOrders{}, &opened).SchoolCaterer(context.Background())
	require.NoError(t, err)
	require.NotNil(t, page.Supply)
	assert.Equal(t, "Küche am Markt", page.Supply.CatererName)
	assert.Equal(t, "Hof", page.Supply.DeliveryAddress)
	assert.Equal(t, supplyStart, page.Supply.StartedAt)
	assert.Empty(t, page.Invitations, "no invitation needed to name the caterer")

	page, err = newTestPortal(t, &fakeCaterers{}, &fakeOrders{}, &opened).SchoolCaterer(context.Background())
	require.NoError(t, err)
	assert.Nil(t, page.Supply, "no supply, no caterer")
}

// #3873: the caterer's delivery list holds every school it supplies, each
// read in its own transaction, in the fixed columns of the template.
func TestDeliveryExportCoversEverySuppliedSchool(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11, 12}, supply: supplying(7), active: true}
	orders := &fakeOrders{week: mealplan.OrderWeek{
		Detail: "total",
		Days:   []mealplan.OrderDay{{Date: "2026-09-07", Status: mealplan.OrderOrdered, Total: 3}},
	}}
	var opened schoolsOpened
	export, err := newTestPortal(t, caterers, orders, &opened).DeliveryExport(context.Background(), 3, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Equal(t, schoolsOpened{11, 12}, opened)
	assert.Equal(t, [][]string{
		{"07.09.2026", "Schule", "Hof", "Alle", "3", ""},
		{"07.09.2026", "Schule", "Hof", "Alle", "3", ""},
	}, export.Lines)

	school, err := newTestPortal(t, caterers, orders, &opened).SchoolDeliveryExport(context.Background(), "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Len(t, school.Lines, 1, "the school's list is its own")
}

// A school that switched its registration off, or ended the supply during
// the export, drops out of the caterer's delivery list; the export of the
// others goes on.
func TestDeliveryExportSkipsASchoolWithoutOrders(t *testing.T) {
	t.Parallel()
	var opened schoolsOpened
	disabled := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11}, supply: supplying(7), active: true}
	export, err := newTestPortal(t, disabled, &fakeOrders{err: mealplan.ErrRegistrationDisabled}, &opened).DeliveryExport(context.Background(), 3, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Empty(t, export.Lines)
	ended := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11}}
	export, err = newTestPortal(t, ended, &fakeOrders{}, &opened).DeliveryExport(context.Background(), 3, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Empty(t, export.Lines)
}

func TestSchoolDeliveryExportRequiresActiveSupply(t *testing.T) {
	t.Parallel()
	orders := &fakeOrders{week: mealplan.OrderWeek{
		Detail: "total",
		Days:   []mealplan.OrderDay{{Date: "2026-09-07", Status: mealplan.OrderOrdered, Total: 3}},
	}}
	var opened schoolsOpened
	export, err := newTestPortal(t, &fakeCaterers{}, orders, &opened).SchoolDeliveryExport(context.Background(), "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, catererportal.ErrSchoolNotSupplied)
	assert.Empty(t, export.Lines)
	assert.False(t, orders.ordersRead, "an inactive supply must not settle or read orders")
}

func TestPortalRejectsDeletedSchools(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11, 12}, supply: supplying(7), active: true}
	orders := &fakeOrders{week: mealplan.OrderWeek{
		Detail: "total", Days: []mealplan.OrderDay{{Date: "2026-09-07", Status: mealplan.OrderOrdered, Total: 3}},
	}}
	var opened schoolsOpened
	portal := newTestPortal(t, caterers, orders, &opened)
	// The supply survives deletion, but the school directory omits deleted schools.
	portal.deps.SchoolNames = func(context.Context, []int64) (map[int64]string, error) {
		return map[int64]string{11: "Schule"}, nil
	}
	_, err := portal.SchoolOrders(context.Background(), 3, 12, "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, catererportal.ErrSchoolNotSupplied)
	require.ErrorIs(t, portal.ConfirmOrder(context.Background(), 3, 12, 5), catererportal.ErrSchoolNotSupplied)
	assert.Empty(t, opened, "a deleted school is never opened")
	assert.Zero(t, orders.confirmedBy)
	schools, err := portal.Schools(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, []catererportal.School{{ID: 11, Name: "Schule"}}, schools.Schools)
	export, err := portal.DeliveryExport(context.Background(), 3, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Equal(t, schoolsOpened{11}, opened)
	assert.Len(t, export.Lines, 1, "only the live school's delivery is exported")
}

func TestPortalRejectsOrdersWhenSchoolLookupFails(t *testing.T) {
	t.Parallel()
	var opened schoolsOpened
	orders := &fakeOrders{}
	portal := newTestPortal(t, &fakeCaterers{schools: []int64{11}}, orders, &opened)
	lookupErr := errors.New("school directory unavailable")
	portal.deps.SchoolNames = func(context.Context, []int64) (map[int64]string, error) { return nil, lookupErr }
	_, err := portal.SchoolOrders(context.Background(), 3, 11, "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, lookupErr)
	require.ErrorIs(t, portal.ConfirmOrder(context.Background(), 3, 11, 5), lookupErr)
	assert.Empty(t, opened)
	assert.Zero(t, orders.confirmedBy)
}
