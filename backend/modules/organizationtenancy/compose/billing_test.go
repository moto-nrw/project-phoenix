package compose

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// stubBillingCounts answers the owners' live figures. The owners' own rules
// are pinned in School Membership and Device Fleet; these tests pin the
// capture around them.
type stubBillingCounts struct {
	students          map[int64]int
	terminals         map[int64]int
	studentCapturedAt time.Time
	err               error
}

func (s *stubBillingCounts) CountActiveStudentsByTenant(_ context.Context, capturedAt time.Time) (map[int64]int, error) {
	s.studentCapturedAt = capturedAt
	return s.students, s.err
}

func (s *stubBillingCounts) CountActiveTerminalsByTenant(context.Context) (map[int64]int, error) {
	return s.terminals, s.err
}

// billingTestSetup gives the test a disposable database: the capture writes a
// row for every school and the key day is one value for the installation, so
// the behavior is database-global.
func billingTestSetup(t *testing.T, counts *stubBillingCounts, now time.Time) (*bun.DB, organizationtenancy.BillingReport, int64) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	billing, err := NewBilling(BillingDependencies{Counts: counts, Now: func() time.Time { return now }})
	require.NoError(t, err)
	return db, billing, schoolID
}

func adminCtx(t *testing.T, db *bun.DB) context.Context {
	t.Helper()
	return testpkg.WithTenantRuntime(t, context.Background(), db)
}

func berlinTime(year int, month time.Month, day, hour, minute int) time.Time {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return time.Date(year, month, day, hour, minute, 0, 0, berlin)
}

func countOf(t *testing.T, counts []organizationtenancy.BillingKeyDateCount, schoolID int64, period string) organizationtenancy.BillingKeyDateCount {
	t.Helper()
	for _, count := range counts {
		if count.SchoolID == schoolID && count.Period == period {
			return count
		}
	}
	t.Fatalf("no captured row for school %d in %s", schoolID, period)
	return organizationtenancy.BillingKeyDateCount{}
}

// TestBillingCapturesOnceOnTheKeyDate pins the capture contract of #2791: no
// row before the key date's capture hour, one row per school on it, and the
// captured figures never change on a later tick, even when the live counts
// do.
func TestBillingCapturesOnceOnTheKeyDate(t *testing.T) {
	t.Parallel()
	counts := &stubBillingCounts{}
	db, billing, schoolID := billingTestSetup(t, counts, berlinTime(2099, time.May, 1, 12, 0))
	counts.students = map[int64]int{schoolID: 7}
	counts.terminals = map[int64]int{schoolID: 2}
	ctx := adminCtx(t, db)

	keyDay, err := billing.SetBillingKeyDay(ctx, 10, testpkg.UniqueTestTenantID(t))
	require.NoError(t, err)
	assert.Equal(t, 10, keyDay.Day)
	assert.Equal(t, "2099-05-10", keyDay.NextKeyDate)

	written, err := billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.May, 10, 5, 59))
	require.NoError(t, err)
	assert.Zero(t, written, "nothing is due before the capture hour")

	written, err = billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.May, 10, 6, 0))
	require.NoError(t, err)
	assert.Positive(t, written)

	counts.students[schoolID] = 9
	counts.terminals[schoolID] = 5
	written, err = billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.May, 20, 6, 0))
	require.NoError(t, err)
	assert.Zero(t, written, "a captured month is never captured again")

	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	row := countOf(t, rows, schoolID, "2099-05-01")
	assert.Equal(t, "2099-05-10", row.KeyDate)
	assert.Equal(t, 7, row.ActiveStudents)
	assert.Equal(t, 2, row.ActiveTerminals)
	assert.NotEmpty(t, row.SchoolName)
	assert.NotEmpty(t, row.OrganizationName)
}

// TestBillingCaptureSkipsSchoolsCreatedAfterTheKeyDate pins that a school
// that did not exist on the key date gets no row for that month, and a
// deleted school gets none either.
func TestBillingCaptureSkipsSchoolsCreatedAfterTheKeyDate(t *testing.T) {
	t.Parallel()
	counts := &stubBillingCounts{}
	db, billing, schoolID := billingTestSetup(t, counts, berlinTime(2099, time.June, 1, 12, 0))
	lateSchoolID, _ := testpkg.CreateTestTenant(t, db)
	deletedSchoolID, _ := testpkg.CreateTestTenant(t, db)
	_, err := db.ExecContext(context.Background(), "UPDATE platform.schools SET created_at = ? WHERE id = ?", berlinTime(2099, time.June, 16, 0, 30), lateSchoolID)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), "UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?", deletedSchoolID)
	require.NoError(t, err)
	ctx := adminCtx(t, db)

	_, err = billing.SetBillingKeyDay(ctx, 15, testpkg.UniqueTestTenantID(t))
	require.NoError(t, err)
	// The worker was down on the 15th: the capture on the 20th is late but
	// keeps the key date, and its time says so.
	_, err = billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.June, 20, 9, 0))
	require.NoError(t, err)
	assert.Equal(t, berlinTime(2099, time.June, 20, 9, 0), counts.studentCapturedAt)

	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	row := countOf(t, rows, schoolID, "2099-06-01")
	assert.Equal(t, "2099-06-15", row.KeyDate)
	assert.Zero(t, row.ActiveStudents, "a school without active children counts zero")
	for _, count := range rows {
		assert.NotEqual(t, lateSchoolID, count.SchoolID, "school created after the key date")
		assert.NotEqual(t, deletedSchoolID, count.SchoolID, "deleted school")
	}
}

// TestBillingCaptureDoesNotBackdateAChangedKeyDay keeps a setting changed
// after its newly selected capture time from creating a made-up past record.
func TestBillingCaptureDoesNotBackdateAChangedKeyDay(t *testing.T) {
	t.Parallel()
	db, billing, _ := billingTestSetup(t, &stubBillingCounts{}, berlinTime(2099, time.September, 1, 12, 0))
	ctx := adminCtx(t, db)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.billing_settings
		SET key_day = 10, updated_at = ? WHERE id = 1`, berlinTime(2099, time.September, 20, 9, 0))
	require.NoError(t, err)

	written, err := billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.September, 20, 9, 0))
	require.NoError(t, err)
	assert.Zero(t, written)
}

func TestBillingSeedCaptureWritesOneCurrentDaySnapshot(t *testing.T) {
	t.Parallel()
	counts := &stubBillingCounts{}
	db, billing, schoolID := billingTestSetup(t, counts, berlinTime(2099, time.September, 1, 12, 0))
	counts.students = map[int64]int{schoolID: 7}
	counts.terminals = map[int64]int{schoolID: 2}
	ctx := adminCtx(t, db)

	written, err := billing.SeedBillingKeyDateCounts(ctx, berlinTime(2099, time.September, 3, 9, 0))
	require.NoError(t, err)
	assert.Positive(t, written)

	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	row := countOf(t, rows, schoolID, "2099-09-01")
	assert.Equal(t, "2099-09-03", row.KeyDate)
	assert.Equal(t, 7, row.ActiveStudents)
	assert.Equal(t, 2, row.ActiveTerminals)

	written, err = billing.SeedBillingKeyDateCounts(ctx, berlinTime(2099, time.September, 3, 10, 0))
	require.NoError(t, err)
	assert.Zero(t, written, "the synthetic demo snapshot is immutable too")
}

// heldBillingCounts holds the first capture after it has listed the schools,
// until the test releases it. The other captures answer at once.
type heldBillingCounts struct {
	listed  chan struct{}
	release chan struct{}
	called  atomic.Bool
}

func (h *heldBillingCounts) CountActiveStudentsByTenant(context.Context, time.Time) (map[int64]int, error) {
	if !h.called.Swap(true) {
		close(h.listed)
		<-h.release
	}
	return nil, nil
}

func (h *heldBillingCounts) CountActiveTerminalsByTenant(context.Context) (map[int64]int, error) {
	return nil, nil
}

// TestBillingSeedCapturesSideBySide pins #5151: the demo process seeds
// several schools at once, and each seed captures the snapshot of every
// school still missing one. A capture that listed a school before another
// capture wrote that school's row skips the row instead of failing, which
// would rebuild the demo school.
func TestBillingSeedCapturesSideBySide(t *testing.T) {
	t.Parallel()
	counts := &heldBillingCounts{listed: make(chan struct{}), release: make(chan struct{})}
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.CreateTestTenant(t, db)
	billing, err := NewBilling(BillingDependencies{Counts: counts, Now: time.Now})
	require.NoError(t, err)
	ctx := adminCtx(t, db)
	now := berlinTime(2099, time.October, 27, 23, 23)

	held := make(chan error, 1)
	go func() {
		_, err := billing.SeedBillingKeyDateCounts(ctx, now)
		held <- err
	}()
	select {
	case <-counts.listed:
	case err := <-held:
		t.Fatalf("the first capture ended before it counted: %v", err)
	}
	written, err := billing.SeedBillingKeyDateCounts(ctx, now)
	close(counts.release)
	require.NoError(t, err)
	assert.Positive(t, written)
	require.NoError(t, <-held, "the held capture finds the rows the other one wrote")
}

// TestBillingCaptureWritesNothingWhenACountFails pins that a failing owner
// count aborts the whole capture instead of writing partial zeros.
func TestBillingCaptureWritesNothingWhenACountFails(t *testing.T) {
	t.Parallel()
	failure := errors.New("membership unavailable")
	counts := &stubBillingCounts{err: failure}
	db, billing, _ := billingTestSetup(t, counts, berlinTime(2099, time.July, 1, 12, 0))
	ctx := adminCtx(t, db)

	_, err := billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.July, 20, 9, 0))
	require.ErrorIs(t, err, failure)
	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// TestBillingKeyDayRejectsDaysNotInEveryMonth pins the 1..28 range.
func TestBillingKeyDayRejectsDaysNotInEveryMonth(t *testing.T) {
	t.Parallel()
	db, billing, _ := billingTestSetup(t, &stubBillingCounts{}, berlinTime(2099, time.August, 20, 12, 0))
	ctx := adminCtx(t, db)

	initial, err := billing.BillingKeyDay(ctx)
	require.NoError(t, err)
	assert.Equal(t, 15, initial.Day, "the migration seeds the 15th")
	assert.Equal(t, "2099-09-15", initial.NextKeyDate)

	for _, day := range []int{0, 29, 31, -3} {
		_, err := billing.SetBillingKeyDay(ctx, day, testpkg.UniqueTestTenantID(t))
		require.ErrorIs(t, err, organizationtenancy.ErrInvalidBillingKeyDay, "day %d", day)
	}
	operatorID := testpkg.UniqueTestTenantID(t)
	updated, err := billing.SetBillingKeyDay(ctx, 28, operatorID)
	require.NoError(t, err)
	assert.Equal(t, 28, updated.Day)
	assert.Equal(t, "2099-08-28", updated.NextKeyDate)
	require.NotNil(t, updated.UpdatedByOperatorID)
	assert.Equal(t, operatorID, *updated.UpdatedByOperatorID)
}

// TestBillingQueryBudget pins that the listing and the capture stay flat in
// the number of schools (#2940).
func TestBillingQueryBudget(t *testing.T) {
	t.Parallel()
	counts := &stubBillingCounts{}
	db, billing, _ := billingTestSetup(t, counts, berlinTime(2099, time.September, 1, 12, 0))
	for range 3 {
		testpkg.CreateTestTenant(t, db)
	}
	counter := testpkg.CaptureQueries(t, db)
	ctx := adminCtx(t, db)

	counter.Reset()
	written, err := billing.RecordDueBillingKeyDates(ctx, berlinTime(2099, time.September, 20, 9, 0))
	require.NoError(t, err)
	require.GreaterOrEqual(t, written, 4)
	testpkg.AssertQueryBudget(t, "modules.organizationtenancy.billing.capture", withoutTransactionControl(counter.Queries()))

	counter.Reset()
	rows, err := billing.ListBillingKeyDateCounts(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 4)
	testpkg.AssertQueryBudget(t, "modules.organizationtenancy.billing.key_date_counts", withoutTransactionControl(counter.Queries()))
}

// withoutTransactionControl drops BEGIN, COMMIT, ROLLBACK and the role and
// setting statements the administrative transaction issues around the
// scenario's own reads and writes.
func withoutTransactionControl(queries []string) []string {
	result := make([]string, 0, len(queries))
	for _, query := range queries {
		upper := strings.ToUpper(strings.TrimSpace(query))
		if strings.HasPrefix(upper, "BEGIN") || strings.HasPrefix(upper, "COMMIT") ||
			strings.HasPrefix(upper, "ROLLBACK") || strings.HasPrefix(upper, "SET ") ||
			strings.HasPrefix(upper, "SELECT SET_CONFIG") {
			continue
		}
		result = append(result, query)
	}
	return result
}

// TestBillingCountsCannotBeChangedByTheServerRoles pins the database side of
// "captured once, never changed": the administrative role may capture through
// the narrow definer writer and read through the live-school view, but not update or delete; request roles
// cannot touch the table. Erased-school evidence needs the separate billing role.
func TestBillingCountsCannotBeChangedByTheServerRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	privileges := map[string]map[string]bool{
		"phoenix_admin":  {"SELECT": false, "INSERT": false, "UPDATE": false, "DELETE": false, "TRUNCATE": false},
		"phoenix_auth":   {"SELECT": false, "INSERT": false, "UPDATE": false, "DELETE": false},
		"phoenix_tenant": {"SELECT": false, "INSERT": false, "UPDATE": false, "DELETE": false},
	}
	for role, expected := range privileges {
		for privilege, allowed := range expected {
			var has bool
			err := db.NewRaw(`SELECT has_table_privilege(?, 'platform.billing_key_date_counts', ?)`, role, privilege).Scan(t.Context(), &has)
			require.NoError(t, err)
			assert.Equal(t, allowed, has, "%s %s", role, privilege)
		}
	}
	for _, role := range []string{"phoenix_admin", "phoenix_auth", "phoenix_tenant", "phoenix_demo", "phoenix_school_erasure", "phoenix_billing_retention"} {
		for _, privilege := range []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var has bool
			require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, 'platform.live_billing_key_date_counts', ?)`, role, privilege).Scan(t.Context(), &has))
			assert.False(t, has, "%s must not inherit %s on the live billing view", role, privilege)
		}
		var capture, readLive, expire bool
		require.NoError(t, db.NewRaw(`SELECT
 has_function_privilege(?, 'platform.capture_billing_key_date_counts(bigint[],text[],text[],text[],text[],integer[],integer[])', 'EXECUTE'),
 has_table_privilege(?, 'platform.live_billing_key_date_counts', 'SELECT'),
 has_function_privilege(?, 'platform.delete_expired_billing_key_date_counts()', 'EXECUTE')`, role, role, role).Scan(t.Context(), &capture, &readLive, &expire))
		assert.Equal(t, role == "phoenix_admin", capture, "%s capture", role)
		assert.Equal(t, role == "phoenix_admin", readLive, "%s regular read", role)
		assert.Equal(t, role == "phoenix_billing_retention", expire, "%s expiry", role)
	}
}

func TestBillingCountsKeepSchoolReferencesAfterHardDeletion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var referencesSchool bool
	err := db.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conrelid = 'platform.billing_key_date_counts'::regclass
		  AND confrelid = 'platform.schools'::regclass
		  AND contype = 'f'
	)`).Scan(t.Context(), &referencesSchool)
	require.NoError(t, err)
	assert.False(t, referencesSchool, "historical billing rows must outlive live school rows")
}
