package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// wishWeek is a published period with one week of capacity one: one placed
// booking, then the waiting bookings of children in the order they came in.
type wishWeek struct {
	organization, school, period, week, model int64
	placedChild                               int64
	waiting                                   []int64 // child IDs in the order they came in
}

func newWishWeek(t *testing.T, db *bun.DB, name string, waitingWishes ...string) wishWeek {
	t.Helper()
	ctx := context.Background()
	w := wishWeek{organization: testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("traeger-wunsch-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID}
	testpkg.OwnTraegerHolidayPeriods(t, db, w.organization)
	w.school = testpkg.CreateTestSchoolInOrganization(t, db, w.organization, name+" OGS")
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2027-01-01', '2027-06-01', 'published', NOW())
		RETURNING id`, w.organization).Scan(ctx, &w.period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id, capacity)
		VALUES (?, ?, '2027-07-05', '2027-07-09', ?, 1) RETURNING id`, w.organization, w.period, w.school).Scan(ctx, &w.week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, w.organization, w.week).Scan(ctx, &w.model))
	w.placedChild = w.book(t, db, 0, "placed", "")
	for i, wish := range waitingWishes {
		w.waiting = append(w.waiting, w.book(t, db, i+1, "waitlisted", wish))
	}
	return w
}

// book adds a registration that came in minute minutes after the first with
// one child of the school and its booking; wish "" is unknown.
func (w wishWeek) book(t *testing.T, db *bun.DB, minute int, placement, wish string) int64 {
	t.Helper()
	ctx := context.Background()
	var registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, submitted_at)
		VALUES (?, ?, 'Eva', 'Wunsch', 'eva.wunsch@example.org', '0123', NOW(), '2027-02-01T08:00:00Z'::timestamptz + (? * INTERVAL '1 minute'))
		RETURNING id`, w.organization, w.period, minute).Scan(ctx, &registration))
	var wishValue, source any
	if wish != "" {
		wishValue, source = wish, "account"
	}
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date, home_school_id, holiday_wish, holiday_wish_source)
		VALUES (?, ?, ?, 1, ?, 'Wunsch', '2018-03-04', ?, ?, ?) RETURNING id`,
		w.organization, w.period, registration, fmt.Sprintf("Kind%d", minute), w.school, wishValue, source).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement) VALUES (?, ?, ?, ?, ?, ?)`,
		w.organization, w.period, child, w.week, w.model, placement)
	require.NoError(t, err)
	return child
}

func (w wishWeek) setRule(t *testing.T, db *bun.DB, on bool) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE enrollment.holiday_periods SET holiday_wish_rule = ? WHERE id = ?`, on, w.period)
	require.NoError(t, err)
}

func (w wishWeek) placement(t *testing.T, db *bun.DB, child int64) string {
	t.Helper()
	var placement string
	require.NoError(t, db.NewRaw(`SELECT placement FROM enrollment.holiday_registration_bookings WHERE holiday_registration_child_id = ?`, child).
		Scan(context.Background(), &placement))
	return placement
}

// TestHolidayWishOrdersOnlyTheWaitingList pins the precedence of #4703
// inside the places of #4699: a child whose wish is "no" waits behind a
// later child with an unknown wish only while its period takes the wish
// into account, the free place a higher capacity opens goes to the first of
// that order, and the place given before stays. It goes red when the
// waiting list ignores the wish or the period's rule.
func TestHolidayWishOrdersOnlyTheWaitingList(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newWishWeek(t, db, "Träger Ferienwunsch Vorrang", "no", "")
	declined, unknown := w.waiting[0], w.waiting[1]
	store := postgres.NewHolidayRegistrationStore()
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))

	waitlist := func() (order []int64, deferred []bool) {
		t.Helper()
		require.NoError(t, testpkg.WithinTraeger(ctx, w.organization, func(ctx context.Context) error {
			places, err := store.ReadHolidayWeekPlaces(ctx, w.organization, w.period)
			require.NoError(t, err)
			require.Len(t, places, 1)
			for _, entry := range places[0].Waitlist {
				order, deferred = append(order, entry.ChildID), append(deferred, entry.Deferred)
			}
			read, _, err := store.ReadHolidayReview(ctx, w.organization, w.period, 0)
			require.NoError(t, err)
			for _, registration := range read {
				for _, child := range registration.Children {
					if child.ID == order[0] {
						require.NotNil(t, child.Bookings[0].WaitlistPosition)
						assert.Equal(t, 1, *child.Bookings[0].WaitlistPosition, "the review names the same order")
					}
				}
			}
			return nil
		}))
		return order, deferred
	}

	order, deferred := waitlist()
	assert.Equal(t, []int64{declined, unknown}, order, "without the rule the order is the time the registrations came in")
	assert.Equal(t, []bool{false, false}, deferred)

	w.setRule(t, db, true)
	order, deferred = waitlist()
	assert.Equal(t, []int64{unknown, declined}, order, "with the rule a child without a wish waits behind")
	assert.Equal(t, []bool{false, true}, deferred)

	two := 2
	require.NoError(t, testpkg.WithinTraeger(ctx, w.organization, func(ctx context.Context) error {
		require.NoError(t, store.SetHolidayWeekCapacity(ctx, w.organization, w.period, w.week, &two))
		queue, found, err := store.ReadHolidayWeekQueue(ctx, w.organization, w.week)
		require.NoError(t, err)
		require.True(t, found)
		var waiting []enrollment.HolidayQueuedBooking
		for _, booking := range queue.Bookings {
			if !booking.Placed {
				waiting = append(waiting, booking)
			}
		}
		require.Len(t, waiting, 2)
		assert.Equal(t, []int64{unknown, declined}, []int64{waiting[0].ChildID, waiting[1].ChildID},
			"the placement reads the child with precedence first")
		placed, err := store.PlaceHolidayBookings(ctx, w.organization, w.week, []int64{waiting[0].ID}, nil)
		require.NoError(t, err)
		require.Len(t, placed, 1)
		assert.Equal(t, unknown, placed[0].ChildID, "the free place goes to the child with precedence")
		return nil
	}))
	assert.Equal(t, "placed", w.placement(t, db, w.placedChild), "the place given before stays")
	assert.Equal(t, "waitlisted", w.placement(t, db, declined))
}

// TestHolidayWishStatementsKeepTheirTenantBeneathRLS pins the tenant filters
// of the holiday care wish (#4703) beneath RLS: as the owner of the tables,
// school B reads no approved wish of a child of school A, and Träger B
// writes no wish of a child of Träger A. Each goes red when its statement
// drops its tenant_id or organization_id filter.
func TestHolidayWishStatementsKeepTheirTenantBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolA := testpkg.Tenant(t)
	student := approvedWithWish(t, db, true)
	store := postgres.NewStore(postgres.NewDatabase(), func(ctx context.Context) (int64, error) { return tenant.FromContext(ctx), nil })
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	on := calendar.NewDate(2027, 1, 4)

	read := func(school int64, beneathRLS bool) *bool {
		t.Helper()
		id, err := tenant.NewTenantID(school)
		require.NoError(t, err)
		var wish *bool
		require.NoError(t, tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
			if beneathRLS {
				require.NoError(t, testpkg.WithoutRequestRole(txCtx))
			}
			wish, err = store.ApprovedHolidayCareWish(txCtx, student, on)
			return err
		}))
		return wish
	}
	require.NotNil(t, read(schoolA, false), "school A reads its child's approved wish")
	assert.True(t, *read(schoolA, false))
	other := testpkg.CreateTestSchoolInOrganization(t, db, testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Wunsch fremd", Slug: fmt.Sprintf("traeger-wunsch-fremd-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID, "OGS fremd")
	assert.Nil(t, read(other, true), "beneath RLS school B reads no wish of school A")

	own := newWishWeek(t, db, "Träger Wunsch A", "")
	foreign := newWishWeek(t, db, "Träger Wunsch B", "")
	holidays := postgres.NewHolidayRegistrationStore()
	runtime := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	var foreignRegistration int64
	require.NoError(t, db.NewRaw(`SELECT holiday_registration_id FROM enrollment.holiday_registration_children WHERE id = ?`, foreign.waiting[0]).
		Scan(context.Background(), &foreignRegistration))
	require.NoError(t, testpkg.WithinTraeger(runtime, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		written, err := holidays.SetHolidayChildWish(ctx, own.organization, foreignRegistration, foreign.waiting[0], "yes", "office")
		require.NoError(t, err)
		assert.False(t, written, "Träger A writes no wish of Träger B")
		return nil
	}))
	var wish *string
	require.NoError(t, db.NewRaw(`SELECT holiday_wish FROM enrollment.holiday_registration_children WHERE id = ?`, foreign.waiting[0]).
		Scan(context.Background(), &wish))
	assert.Nil(t, wish)
}

// TestApprovedHolidayCareWishCountsOnlyTheApprovedSchoolYear pins which
// answer counts (#4703): the approved enrollment of the school year that
// holds the day, never a submitted or rejected one and never another year.
func TestApprovedHolidayCareWishCountsOnlyTheApprovedSchoolYear(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := postgres.NewStore(postgres.NewDatabase(), func(ctx context.Context) (int64, error) { return tenant.FromContext(ctx), nil })
	ctx := testpkg.Ctx(t)
	read := func(student int64, on calendar.Date) *bool {
		t.Helper()
		var wish *bool
		id, err := tenant.NewTenantID(testpkg.Tenant(t))
		require.NoError(t, err)
		require.NoError(t, tenant.WithinTenant(tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db)), id, func(txCtx context.Context) error {
			wish, err = store.ApprovedHolidayCareWish(txCtx, student, on)
			return err
		}))
		return wish
	}
	today := calendar.NewDate(2027, 1, 4)
	approved := approvedWithWish(t, db, false)
	require.NotNil(t, read(approved, today))
	assert.False(t, *read(approved, today), "an approved no counts as no")
	assert.Nil(t, read(approved, today.AddDays(400)), "another school year has no wish")

	submitted := approvedWithWish(t, db, true)
	_, err := db.ExecContext(context.Background(), `UPDATE enrollment.request_children SET status = 'submitted' WHERE created_student_id = ?`, submitted)
	require.NoError(t, err)
	assert.Nil(t, read(submitted, today), "an undecided enrollment does not count")
}

func TestApprovedHolidayCareWishIgnoresOfficeMatchedFamilies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := approvedWithWish(t, db, true)
	foreign := approvedWithWish(t, db, false)
	ctx := testpkg.Ctx(t)
	_, err := db.ExecContext(ctx, `UPDATE enrollment.request_children
		SET matched_student_id = ?, created_student_id = NULL, matched_by_office = TRUE, reviewed_at = NOW()
		WHERE created_student_id = ?`, student, foreign)
	require.NoError(t, err)
	store := postgres.NewStore(postgres.NewDatabase(), func(ctx context.Context) (int64, error) { return tenant.FromContext(ctx), nil })
	read := func() *bool {
		t.Helper()
		var wish *bool
		id, err := tenant.NewTenantID(testpkg.Tenant(t))
		require.NoError(t, err)
		require.NoError(t, tenant.WithinTenant(tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db)), id, func(txCtx context.Context) error {
			wish, err = store.ApprovedHolidayCareWish(txCtx, student, calendar.NewDate(2027, 1, 4))
			return err
		}))
		return wish
	}
	wish := read()
	require.NotNil(t, wish)
	assert.True(t, *wish, "a newer office match must not override the authorized family's answer")

	_, err = db.ExecContext(ctx, `UPDATE enrollment.request_children SET status = 'rejected' WHERE created_student_id = ?`, student)
	require.NoError(t, err)
	assert.Nil(t, read(), "an office match alone provides no trusted answer")

	_, err = db.ExecContext(ctx, `UPDATE enrollment.request_children SET matched_by_office = FALSE WHERE matched_student_id = ?`, student)
	require.NoError(t, err)
	wish = read()
	require.NotNil(t, wish, "an authorized re-enrollment still provides an answer")
	assert.False(t, *wish)
}

// The newest approved enrollment decides (#4703). When it did not ask, the
// wish is unknown; an older answer must not become a proof that refuses.
func TestApprovedHolidayCareWishTakesTheNewestEnrollmentEvenWithoutAnAnswer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := approvedWithWish(t, db, false)
	_, err := db.ExecContext(ctx, `UPDATE enrollment.request_children SET reviewed_at = NOW() - INTERVAL '30 days'
		WHERE created_student_id = ?`, student)
	require.NoError(t, err)
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, db, calendar.NewDate(2027, 1, 4))
	request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, "wunsch@example.org", fmt.Sprintf("wish-new-%d", testpkg.UniqueTestTenantID(t)))
	_, err = db.ExecContext(ctx, `INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, status, matched_student_id, reviewed_at)
		VALUES (?, ?, 'Wanda', 'Wunsch', '2018-03-04', 'approved', ?, NOW())`, testpkg.Tenant(t), request.ID, student)
	require.NoError(t, err)

	store := postgres.NewStore(postgres.NewDatabase(), func(ctx context.Context) (int64, error) { return tenant.FromContext(ctx), nil })
	var wish *bool
	id, err := tenant.NewTenantID(testpkg.Tenant(t))
	require.NoError(t, err)
	require.NoError(t, tenant.WithinTenant(tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, db)), id, func(txCtx context.Context) error {
		wish, err = store.ApprovedHolidayCareWish(txCtx, student, calendar.NewDate(2027, 1, 4))
		return err
	}))
	assert.Nil(t, wish, "the newest enrollment did not ask, so the older no does not count")
}

// approvedWithWish approves a child of the test tenant in a school-year
// phase around 04.01.2027 with the holiday care wish and returns its student.
func approvedWithWish(t *testing.T, db *bun.DB, wish bool) int64 {
	t.Helper()
	phase := testpkg.CreateTestEnrollmentPhaseAround(t, db, calendar.NewDate(2027, 1, 4))
	request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, "wunsch@example.org", fmt.Sprintf("wish-%d", testpkg.UniqueTestTenantID(t)))
	student := testpkg.CreateTestStudent(t, db, "Wanda", "Wunsch", "2a")
	_, err := db.ExecContext(context.Background(), `INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, status, created_student_id, holiday_care_wish)
		VALUES (?, ?, 'Wanda', 'Wunsch', '2018-03-04', 'approved', ?, ?)`, testpkg.Tenant(t), request.ID, student.ID, wish)
	require.NoError(t, err)
	return student.ID
}
