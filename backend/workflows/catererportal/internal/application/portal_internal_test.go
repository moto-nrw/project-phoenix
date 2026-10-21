package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
)

type fakeCaterers struct {
	caterer     identityaccess.Caterer
	schools     []int64
	invitations []identityaccess.CatererInvitation
	acceptance  identityaccess.CatererAcceptance
	linkEnded   bool
	err         error
}

func (f *fakeCaterers) CreateCatererInvitation(context.Context, identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error) {
	return identityaccess.CatererInvitation{}, f.err
}
func (f *fakeCaterers) ListCatererInvitations(context.Context) ([]identityaccess.CatererInvitation, error) {
	return f.invitations, f.err
}
func (f *fakeCaterers) RevokeCatererInvitation(context.Context, int64) error { return f.err }
func (f *fakeCaterers) PreviewCatererInvitation(context.Context, string) (identityaccess.CatererInvitationPreview, error) {
	return identityaccess.CatererInvitationPreview{}, f.err
}
func (f *fakeCaterers) AcceptCatererInvitation(ctx context.Context, _ string, _ identityaccess.CatererRegistration, start identityaccess.CatererAcceptanceHook) (identityaccess.CatererAcceptance, error) {
	if f.err != nil {
		return identityaccess.CatererAcceptance{}, f.err
	}
	return f.acceptance, start(ctx, f.acceptance)
}
func (f *fakeCaterers) EndCatererSchool(context.Context) (bool, error) {
	f.linkEnded = true
	return true, f.err
}
func (f *fakeCaterers) SchoolCaterer(context.Context) (identityaccess.Caterer, bool, error) {
	return f.caterer, f.caterer.ID > 0, f.err
}
func (f *fakeCaterers) CatererSchools(context.Context, int64) (identityaccess.Caterer, []int64, error) {
	return f.caterer, f.schools, f.err
}
func (f *fakeCaterers) LoginCatererWithAudit(context.Context, string, string, string, string) (string, string, error) {
	return "access", "refresh", f.err
}

type fakeSupplies struct {
	supply      mealplan.Supply
	active      bool
	started     []int64
	ended       bool
	read        int64
	ordersRead  bool
	confirmedBy int64
	week        mealplan.OrderWeek
	err         error
}

func (f *fakeSupplies) StartSupply(_ context.Context, catererID int64, _ string, _ int64) error {
	f.started = append(f.started, catererID)
	return f.err
}
func (f *fakeSupplies) EndSupply(context.Context, int64) (bool, error) {
	f.ended = true
	return f.active, f.err
}
func (f *fakeSupplies) CurrentSupply(context.Context) (mealplan.Supply, bool, error) {
	return f.supply, f.active, f.err
}
func (f *fakeSupplies) CatererOrders(_ context.Context, catererID int64, _, _ mealplan.Date) (mealplan.OrderWeek, error) {
	f.read = catererID
	if f.week.Days != nil {
		return f.week, f.err
	}
	return mealplan.OrderWeek{Rhythm: "daily"}, f.err
}
func (f *fakeSupplies) Orders(context.Context, mealplan.Date, mealplan.Date) (mealplan.OrderWeek, error) {
	f.ordersRead = true
	return f.week, f.err
}

func (f *fakeSupplies) ConfirmOrder(_ context.Context, _, _, accountID int64) error {
	f.confirmedBy = accountID
	return f.err
}

// schoolsOpened records each school transaction the portal opens.
type schoolsOpened []int64

func (s *schoolsOpened) within(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	*s = append(*s, schoolID)
	return fn(ctx)
}

func newTestPortal(t *testing.T, caterers *fakeCaterers, supplies *fakeSupplies, opened *schoolsOpened) *Portal {
	t.Helper()
	portal, err := New(Dependencies{
		Caterers: caterers, Supplies: supplies, WithinSchool: opened.within,
		CurrentSchool: func(context.Context) int64 { return 11 },
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

// ADR 0077: the caterer reads and confirms only in a school it supplies, and
// each school in its own transaction.
func TestPortalReadsOnlySuppliedSchools(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7, Name: "Küche"}, schools: []int64{11, 12}}
	supplies := &fakeSupplies{supply: mealplan.Supply{CatererID: 7, DeliveryAddress: "Hof"}, active: true}
	var opened schoolsOpened
	portal := newTestPortal(t, caterers, supplies, &opened)

	_, err := portal.SchoolOrders(context.Background(), 3, 99, "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, catererportal.ErrSchoolNotSupplied)
	assert.Empty(t, opened, "a foreign school is never opened")
	require.ErrorIs(t, portal.ConfirmOrder(context.Background(), 3, 99, 5), catererportal.ErrSchoolNotSupplied)

	orders, err := portal.SchoolOrders(context.Background(), 3, 12, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Equal(t, schoolsOpened{12}, opened)
	assert.Equal(t, int64(7), supplies.read, "the Meal Plan checks the same caterer again")
	assert.Equal(t, "Hof", orders.DeliveryAddress)
	require.NoError(t, portal.ConfirmOrder(context.Background(), 3, 11, 5))
	assert.Equal(t, int64(3), supplies.confirmedBy, "the caterer's account confirms")

	schools, err := portal.Schools(context.Background(), 3)
	require.NoError(t, err)
	assert.Equal(t, catererportal.CatererSchools{CatererName: "Küche", Schools: []catererportal.School{{ID: 11, Name: "Schule"}, {ID: 12, Name: "Schule"}}}, schools)
}

// ADR 0077: the acceptance starts the supply in the school the link names,
// with the caterer it created.
func TestPortalAcceptanceStartsTheSupplyInTheLinkedSchool(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{acceptance: identityaccess.CatererAcceptance{AccountID: 3, CatererID: 7, TenantID: 11}}
	supplies := &fakeSupplies{}
	var opened schoolsOpened
	require.NoError(t, newTestPortal(t, caterers, supplies, &opened).AcceptInvitation(context.Background(), 11, "token", identityaccess.CatererRegistration{}))
	assert.Equal(t, schoolsOpened{11}, opened)
	assert.Equal(t, []int64{7}, supplies.started)
}

// ADR 0077: ending the supply also ends the caterer's link to the school;
// a failure of either leaves the caller's transaction to roll back.
func TestPortalEndsSupplyAndLinkTogether(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{}
	supplies := &fakeSupplies{active: true}
	var opened schoolsOpened
	ended, err := newTestPortal(t, caterers, supplies, &opened).EndSupply(context.Background(), 1)
	require.NoError(t, err)
	assert.True(t, ended)
	assert.True(t, supplies.ended)
	assert.True(t, caterers.linkEnded)

	failing := &fakeSupplies{err: errors.New("database down")}
	_, err = newTestPortal(t, &fakeCaterers{}, failing, &opened).EndSupply(context.Background(), 1)
	require.Error(t, err)
}

// The school's page names the caterer of the running supply from its link
// to the school, not from an invitation the retention may have deleted.
func TestSchoolCatererNamesTheSupplyingCaterer(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7, Name: "Küche am Markt"}}
	var opened schoolsOpened
	page, err := newTestPortal(t, caterers, &fakeSupplies{active: true, supply: mealplan.Supply{DeliveryAddress: "Hof"}}, &opened).SchoolCaterer(context.Background())
	require.NoError(t, err)
	require.NotNil(t, page.Supply)
	assert.Equal(t, "Küche am Markt", page.Supply.CatererName)
	assert.Equal(t, "Hof", page.Supply.DeliveryAddress)
	assert.Empty(t, page.Invitations, "no invitation needed to name the caterer")

	page, err = newTestPortal(t, caterers, &fakeSupplies{}, &opened).SchoolCaterer(context.Background())
	require.NoError(t, err)
	assert.Nil(t, page.Supply, "no supply, no caterer")
}

// #3873: the caterer's delivery list holds every school it supplies, each
// read in its own transaction, in the fixed columns of the template.
func TestDeliveryExportCoversEverySuppliedSchool(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11, 12}}
	supplies := &fakeSupplies{active: true, supply: mealplan.Supply{DeliveryAddress: "Hof"}, week: mealplan.OrderWeek{
		Detail: "total",
		Days:   []mealplan.OrderDay{{Date: "2026-09-07", Status: mealplan.OrderOrdered, Total: 3}},
	}}
	var opened schoolsOpened
	export, err := newTestPortal(t, caterers, supplies, &opened).DeliveryExport(context.Background(), 3, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Equal(t, schoolsOpened{11, 12}, opened)
	assert.Equal(t, [][]string{
		{"07.09.2026", "Schule", "Hof", "Alle", "3", ""},
		{"07.09.2026", "Schule", "Hof", "Alle", "3", ""},
	}, export.Lines)

	school, err := newTestPortal(t, caterers, supplies, &opened).SchoolDeliveryExport(context.Background(), "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Len(t, school.Lines, 1, "the school's list is its own")
}

// A school that switched its registration off drops out of the caterer's
// delivery list; the export of the others goes on.
func TestDeliveryExportSkipsASchoolWithoutOrders(t *testing.T) {
	t.Parallel()
	caterers := &fakeCaterers{caterer: identityaccess.Caterer{ID: 7}, schools: []int64{11}}
	supplies := &fakeSupplies{err: mealplan.ErrRegistrationDisabled}
	var opened schoolsOpened
	export, err := newTestPortal(t, caterers, supplies, &opened).DeliveryExport(context.Background(), 3, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.Empty(t, export.Lines)
}

func TestSchoolDeliveryExportRequiresActiveSupply(t *testing.T) {
	t.Parallel()
	supplies := &fakeSupplies{week: mealplan.OrderWeek{
		Detail: "total",
		Days:   []mealplan.OrderDay{{Date: "2026-09-07", Status: mealplan.OrderOrdered, Total: 3}},
	}}
	var opened schoolsOpened
	export, err := newTestPortal(t, &fakeCaterers{}, supplies, &opened).SchoolDeliveryExport(context.Background(), "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, mealplan.ErrNotSupplied)
	assert.Empty(t, export.Lines)
	assert.False(t, supplies.ordersRead, "an inactive supply must not settle or read orders")
}
